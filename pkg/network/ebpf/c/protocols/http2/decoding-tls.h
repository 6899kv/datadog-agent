#ifndef __HTTP2_DECODING_TLS_H
#define __HTTP2_DECODING_TLS_H

#include "protocols/http2/decoding-common.h"
#include "protocols/http2/usm-events.h"
#include "protocols/http/types.h"

READ_INTO_USER_BUFFER_WITHOUT_TELEMETRY(http2_preface, HTTP2_MARKER_SIZE)
READ_INTO_USER_BUFFER_WITHOUT_TELEMETRY(http2_frame_header, HTTP2_FRAME_HEADER_SIZE)
READ_INTO_USER_BUFFER_WITHOUT_TELEMETRY(http2_path, HTTP2_MAX_PATH_LEN)

// Similar to tls_read_hpack_int, but with a small optimization of getting the
// current character as input argument.
static __always_inline bool tls_read_hpack_int_with_given_current_char(tls_dispatcher_arguments_t *info, __u64 current_char_as_number, __u64 max_number_for_bits, __u64 *out) {
    current_char_as_number &= max_number_for_bits;

    // In HPACK, if the number is too big to be stored in max_number_for_bits
    // bits, then those bits are all set to one, and the rest of the number must
    // be read from subsequent bytes.
    if (current_char_as_number < max_number_for_bits) {
        *out = current_char_as_number;
        return true;
    }

    // Read the next byte, and check if it is the last byte of the number.
    // While HPACK does support arbitrary sized numbers, we are limited by the
    // number of instructions we can use in a single eBPF program, so we only
    // parse one additional byte. The max value that can be parsed is
    // `(2^max_number_for_bits - 1) + 127`.
    __u64 next_char = 0;
    if (bpf_probe_read_user(&next_char, sizeof(__u8), info->buffer_ptr + info->off) >= 0 && (next_char & 128) == 0) {
        info->off++;
        *out = current_char_as_number + (next_char & 127);
        return true;
    }

    return false;
}

// tls_read_hpack_int reads an unsigned variable length integer as specified in the
// HPACK specification, from an skb.
//
// See https://httpwg.org/specs/rfc7541.html#rfc.section.5.1 for more details on
// how numbers are represented in HPACK.
//
// max_number_for_bits represents the number of bits in the first byte that are
// used to represent the MSB of number. It must always be between 1 and 8.
//
// The parsed number is stored in out.
//
// read_hpack_int returns true if the integer was successfully parsed, and false
// otherwise.
static __always_inline bool tls_read_hpack_int(tls_dispatcher_arguments_t *info, __u64 max_number_for_bits, __u64 *out, bool *is_huffman_encoded) {
    __u64 current_char_as_number = 0;
    if (bpf_probe_read_user(&current_char_as_number, sizeof(__u8), info->buffer_ptr + info->off) < 0) {
        return false;
    }
    info->off++;
    // We are only interested in the first bit of the first byte, which indicates if it is huffman encoded or not.
    // See: https://datatracker.ietf.org/doc/html/rfc7541#appendix-B for more details on huffman code.
    *is_huffman_encoded = (current_char_as_number & 128) > 0;

    return tls_read_hpack_int_with_given_current_char(info, current_char_as_number, max_number_for_bits, out);
}

// tls_parse_field_literal parses a header with a literal value.
//
// We are only interested in path headers, that we will store in our internal
// dynamic table, and will skip headers that are not path headers.
static __always_inline bool tls_parse_field_literal(tls_dispatcher_arguments_t *info, http2_header_t *headers_to_process, __u64 index, __u64 global_dynamic_counter, __u8 *interesting_headers_counter, http2_telemetry_t *http2_tel) {
    __u64 str_len = 0;
    bool is_huffman_encoded = false;
    // String length supposed to be represented with at least 7 bits representation -https://datatracker.ietf.org/doc/html/rfc7541#section-5.2
    if (!tls_read_hpack_int(info, MAX_7_BITS, &str_len, &is_huffman_encoded)) {
        return false;
    }

    // The header name is new and inserted in the dynamic table - we skip the new value.
    if (index == 0) {
        info->off += str_len;
        str_len = 0;
        // String length supposed to be represented with at least 7 bits representation -https://datatracker.ietf.org/doc/html/rfc7541#section-5.2
        // At this point the huffman code is not interesting due to the fact that we already read the string length,
        // We are reading the current size in order to skip it.
        if (!tls_read_hpack_int(info, MAX_7_BITS, &str_len, &is_huffman_encoded)) {
            return false;
        }
        goto end;
    }

    if (index == kIndexPath) {
        update_path_size_telemetry(http2_tel, str_len);
    } else {
        goto end;
    }

    // We skip if:
    // - The string is too big
    // - This is not a path
    // - We won't be able to store the header info
    if (headers_to_process == NULL) {
        goto end;
    }

    if (info->off + str_len > info->len) {
        __sync_fetch_and_add(&http2_tel->path_exceeds_frame, 1);
        goto end;
    }

    headers_to_process->index = global_dynamic_counter - 1;
    headers_to_process->type = kNewDynamicHeader;
    headers_to_process->new_dynamic_value_offset = info->off;
    headers_to_process->new_dynamic_value_size = str_len;
    headers_to_process->is_huffman_encoded = is_huffman_encoded;
    // If the string len (`str_len`) is in the range of [0, HTTP2_MAX_PATH_LEN], and we don't exceed packet boundaries
    // (info->off + str_len <= info->len) and the index is kIndexPath, then we have a path header,
    // and we're increasing the counter. In any other case, we're not increasing the counter.
    *interesting_headers_counter += (str_len > 0 && str_len <= HTTP2_MAX_PATH_LEN);
end:
    info->off += str_len;
    return true;
}

// tls_filter_relevant_headers parses the http2 headers frame, and filters headers
// that are relevant for us, to be processed later on.
// The return value is the number of relevant headers that were found and inserted
// in the `headers_to_process` table.
static __always_inline __u8 tls_filter_relevant_headers(tls_dispatcher_arguments_t *info, dynamic_table_index_t *dynamic_index, http2_header_t *headers_to_process, __u32 frame_length, http2_telemetry_t *http2_tel) {
    __u8 current_ch;
    __u8 interesting_headers = 0;
    http2_header_t *current_header;
    const __u32 frame_end = info->off + frame_length;
    const __u32 end = frame_end < info->len + 1 ? frame_end : info->len + 1;
    bool is_indexed = false;
    bool is_dynamic_table_update = false;
    __u64 max_bits = 0;
    __u64 index = 0;

    __u64 *global_dynamic_counter = get_dynamic_counter(&info->tup);
    if (global_dynamic_counter == NULL) {
        return 0;
    }

#pragma unroll(HTTP2_MAX_HEADERS_COUNT_FOR_FILTERING)
    for (__u8 headers_index = 0; headers_index < HTTP2_MAX_HEADERS_COUNT_FOR_FILTERING; ++headers_index) {
        if (info->off >= end) {
            break;
        }
        bpf_probe_read_user(&current_ch, sizeof(current_ch), info->buffer_ptr + info->off);
        info->off++;

        // To determine the size of the dynamic table update, we read an integer representation byte by byte.
        // We continue reading bytes until we encounter a byte without the Most Significant Bit (MSB) set,
        // indicating that we've consumed the complete integer. While in the context of the dynamic table
        // update, we set the state as true if the MSB is set, and false otherwise. Then, we proceed to the next byte.
        // More on the feature - https://httpwg.org/specs/rfc7541.html#rfc.section.6.3.
        if (is_dynamic_table_update) {
            is_dynamic_table_update = (current_ch & 128) != 0;
            continue;
        }
        // 224 is represented as 0b11100000, which is the OR operation for
        // - indexed representation     (0b10000000)
        // - literal representation     (0b01000000)
        // - dynamic table size update  (0b00100000)
        // Thus current_ch & 224 will be 0 only if the top 3 bits are 0, which means that the current byte is not
        // representing any of the above.
        if ((current_ch & 224) == 0) {
            continue;
        }
        // 32 is represented as 0b00100000, which is the scenario of dynamic table size update.
        // From the previous condition we know that the top 3 bits are not 0, so if the top 3 bits are 001, then
        // we have a dynamic table size update.
        is_dynamic_table_update = (current_ch & 224) == 32;
        if (is_dynamic_table_update) {
            continue;
        }

        is_indexed = (current_ch & 128) != 0;
        max_bits = is_indexed ? MAX_7_BITS : MAX_6_BITS;

        index = 0;
        if (!tls_read_hpack_int_with_given_current_char(info, current_ch, max_bits, &index)) {
            break;
        }

        current_header = NULL;
        if (interesting_headers < HTTP2_MAX_HEADERS_COUNT_FOR_PROCESSING) {
            current_header = &headers_to_process[interesting_headers];
        }

        if (is_indexed) {
            // Indexed representation.
            // MSB bit set.
            // https://httpwg.org/specs/rfc7541.html#rfc.section.6.1
            parse_field_indexed(dynamic_index, current_header, index, *global_dynamic_counter, &interesting_headers);
        } else {
            __sync_fetch_and_add(global_dynamic_counter, 1);
            // 6.2.1 Literal Header Field with Incremental Indexing
            // top two bits are 11
            // https://httpwg.org/specs/rfc7541.html#rfc.section.6.2.1
            if (!tls_parse_field_literal(info, current_header, index, *global_dynamic_counter, &interesting_headers, http2_tel)) {
                break;
            }
        }
    }

    return interesting_headers;
}

// tls_process_headers processes the headers that were filtered in filter_relevant_headers,
// looking for requests path, status code, and method.
static __always_inline void tls_process_headers(tls_dispatcher_arguments_t *info, dynamic_table_index_t *dynamic_index, http2_stream_t *current_stream, http2_header_t *headers_to_process, __u8 interesting_headers,  http2_telemetry_t *http2_tel) {
    http2_header_t *current_header;
    dynamic_table_entry_t dynamic_value = {};

#pragma unroll(HTTP2_MAX_HEADERS_COUNT_FOR_PROCESSING)
    for (__u8 iteration = 0; iteration < HTTP2_MAX_HEADERS_COUNT_FOR_PROCESSING; ++iteration) {
        if (iteration >= interesting_headers) {
            break;
        }

        current_header = &headers_to_process[iteration];

        if (current_header->type == kStaticHeader) {
            if (current_header->index == kPOST || current_header->index == kGET) {
                // TODO: mark request
                current_stream->request_started = bpf_ktime_get_ns();
                current_stream->request_method = current_header->index;
                __sync_fetch_and_add(&http2_tel->request_seen, 1);
            } else if (current_header->index >= k200 && current_header->index <= k500) {
                current_stream->response_status_code = current_header->index;
                __sync_fetch_and_add(&http2_tel->response_seen, 1);
            } else if (current_header->index == kEmptyPath) {
                current_stream->path_size = HTTP2_ROOT_PATH_LEN;
                bpf_memcpy(current_stream->request_path, HTTP2_ROOT_PATH, HTTP2_ROOT_PATH_LEN);
            } else if (current_header->index == kIndexPath) {
                current_stream->path_size = HTTP2_INDEX_PATH_LEN;
                bpf_memcpy(current_stream->request_path, HTTP2_INDEX_PATH, HTTP2_INDEX_PATH_LEN);
            }
            continue;
        }

        dynamic_index->index = current_header->index;
        if (current_header->type == kExistingDynamicHeader) {
            dynamic_table_entry_t *dynamic_value = bpf_map_lookup_elem(&http2_dynamic_table, dynamic_index);
            if (dynamic_value == NULL) {
                break;
            }
            current_stream->path_size = dynamic_value->string_len;
            current_stream->is_huffman_encoded = dynamic_value->is_huffman_encoded;
            bpf_memcpy(current_stream->request_path, dynamic_value->buffer, HTTP2_MAX_PATH_LEN);
        } else {
            dynamic_value.string_len = current_header->new_dynamic_value_size;
            dynamic_value.is_huffman_encoded = current_header->is_huffman_encoded;

            // create the new dynamic value which will be added to the internal table.
            read_into_user_buffer_http2_path(dynamic_value.buffer, info->buffer_ptr + current_header->new_dynamic_value_offset);
            bpf_map_update_elem(&http2_dynamic_table, dynamic_index, &dynamic_value, BPF_ANY);
            current_stream->path_size = current_header->new_dynamic_value_size;
            current_stream->is_huffman_encoded = current_header->is_huffman_encoded;
            bpf_memcpy(current_stream->request_path, dynamic_value.buffer, HTTP2_MAX_PATH_LEN);
        }
    }
}

static __always_inline void tls_process_headers_frame(tls_dispatcher_arguments_t *info, http2_stream_t *current_stream, dynamic_table_index_t *dynamic_index, http2_frame_t *current_frame_header, http2_telemetry_t *http2_tel) {
    const __u32 zero = 0;

    // Allocating an array of headers, to hold all interesting headers from the frame.
    http2_header_t *headers_to_process = bpf_map_lookup_elem(&http2_headers_to_process, &zero);
    if (headers_to_process == NULL) {
        return;
    }
    bpf_memset(headers_to_process, 0, HTTP2_MAX_HEADERS_COUNT_FOR_PROCESSING * sizeof(http2_header_t));

    __u8 interesting_headers = tls_filter_relevant_headers(info, dynamic_index, headers_to_process, current_frame_header->length, http2_tel);
    tls_process_headers(info, dynamic_index, current_stream, headers_to_process, interesting_headers, http2_tel);
}

// tls_skip_preface is a helper function to check for the HTTP2 magic sent at the beginning
// of an HTTP2 connection, and skip it if present.
static __always_inline void tls_skip_preface(tls_dispatcher_arguments_t *info) {
    char preface[HTTP2_MARKER_SIZE];
    bpf_memset((char *)preface, 0, HTTP2_MARKER_SIZE);
    read_into_user_buffer_http2_preface(preface, info->buffer_ptr + info->off);
    if (is_http2_preface(preface, HTTP2_MARKER_SIZE)) {
        info->off += HTTP2_MARKER_SIZE;
    }
}
// The function is trying to read the remaining of a split frame header. We have the first part in
// `frame_state->buf` (from the previous packet), and now we're trying to read the remaining (`frame_state->remainder`
// bytes from the current packet).
static __always_inline void tls_fix_header_frame(tls_dispatcher_arguments_t *info, char *out, frame_header_remainder_t *frame_state) {
    bpf_memcpy(out, frame_state->buf, HTTP2_FRAME_HEADER_SIZE);
    // Verifier is unhappy with a single call to `bpf_skb_load_bytes` with a variable length (although checking boundaries)
    switch (frame_state->remainder) {
    case 1:
        bpf_probe_read_user(out + HTTP2_FRAME_HEADER_SIZE - 1, 1, info->buffer_ptr + info->off);
        break;
    case 2:
        bpf_probe_read_user(out + HTTP2_FRAME_HEADER_SIZE - 2, 2, info->buffer_ptr + info->off);
        break;
    case 3:
        bpf_probe_read_user(out + HTTP2_FRAME_HEADER_SIZE - 3, 3, info->buffer_ptr + info->off);
        break;
    case 4:
        bpf_probe_read_user(out + HTTP2_FRAME_HEADER_SIZE - 4, 4, info->buffer_ptr + info->off);
        break;
    case 5:
        bpf_probe_read_user(out + HTTP2_FRAME_HEADER_SIZE - 5, 5, info->buffer_ptr + info->off);
        break;
    case 6:
        bpf_probe_read_user(out + HTTP2_FRAME_HEADER_SIZE - 6, 6, info->buffer_ptr + info->off);
        break;
    case 7:
        bpf_probe_read_user(out + HTTP2_FRAME_HEADER_SIZE - 7, 7, info->buffer_ptr + info->off);
        break;
    case 8:
        bpf_probe_read_user(out + HTTP2_FRAME_HEADER_SIZE - 8, 8, info->buffer_ptr + info->off);
        break;
    }
    return;
}

static __always_inline bool tls_get_first_frame(tls_dispatcher_arguments_t *info, frame_header_remainder_t *frame_state, http2_frame_t *current_frame, http2_telemetry_t *http2_tel) {
    // No state, try reading a frame.
    if (frame_state == NULL) {
        // Checking we have enough bytes in the packet to read a frame header.
        if (info->off + HTTP2_FRAME_HEADER_SIZE > info->len) {
            // Not enough bytes, cannot read frame, so we have 0 interesting frames in that packet.
            return false;
        }

        // Reading frame, and ensuring the frame is valid.
        read_into_user_buffer_http2_frame_header((char *)current_frame, info->buffer_ptr + info->off);
        info->off += HTTP2_FRAME_HEADER_SIZE;
        if (!format_http2_frame_header(current_frame)) {
            // Frame is not valid, so we have 0 interesting frames in that packet.
            return false;
        }
        return true;
    }

    // Getting here means we have a frame state from the previous packets.
    // Scenarios in order:
    //  1. Check if we have a frame-header remainder - if so, we must try and read the rest of the frame header.
    //     In case of a failure, we abort.
    //  2. If we don't have a frame-header remainder, then we're trying to read a valid frame.
    //     HTTP2 can send valid frames (like SETTINGS and PING) during a split DATA frame. If such a frame exists,
    //     then we won't have the rest of the split frame in the same packet.
    //  3. If we reached here, and we have a remainder, then we're consuming the remainder and checking we can read the
    //     next frame header.
    //  4. We failed reading any frame. Aborting.

    // Frame-header-remainder.
    if (frame_state->header_length > 0) {
        tls_fix_header_frame(info, (char *)current_frame, frame_state);
        if (format_http2_frame_header(current_frame)) {
            info->off += frame_state->remainder;
            frame_state->remainder = 0;
            return true;
        }

        // We couldn't read frame header using the remainder.
        return false;
    }

    // Checking if we can read a frame header.
    if (info->off + HTTP2_FRAME_HEADER_SIZE <= info->len) {
        read_into_user_buffer_http2_frame_header((char *)current_frame, info->buffer_ptr + info->off);
        if (format_http2_frame_header(current_frame)) {
            // We successfully read a valid frame.
            info->off += HTTP2_FRAME_HEADER_SIZE;
            return true;
        }
    }

    // We failed to read a frame, if we have a remainder trying to consume it and read the following frame.
    if (frame_state->remainder > 0) {
        info->off += frame_state->remainder;
        // The remainders "ends" the current packet. No interesting frames were found.
        if (info->off == info->len) {
            frame_state->remainder = 0;
            return false;
        }
        reset_frame(current_frame);
        read_into_user_buffer_http2_frame_header((char *)current_frame, info->buffer_ptr + info->off);
        if (format_http2_frame_header(current_frame)) {
            frame_state->remainder = 0;
            info->off += HTTP2_FRAME_HEADER_SIZE;
            return true;
        }
    }
    // still not valid / does not have a remainder - abort.
    return false;
}

// tls_find_relevant_frames iterates over the packet and finds frames that are
// relevant for us. The frames info and location are stored in the `iteration_value->frames_array` array,
// and the number of frames found is being stored at iteration_value->frames_count.
//
// We consider frames as relevant if they are either:
// - HEADERS frames
// - RST_STREAM frames
// - DATA frames with the END_STREAM flag set
static __always_inline void tls_find_relevant_frames(tls_dispatcher_arguments_t *info, http2_tail_call_state_t *iteration_value, http2_telemetry_t *http2_tel) {
    bool is_headers_or_rst_frame, is_data_end_of_stream;
    http2_frame_t current_frame = {};

   // If we have found enough interesting frames, we should not process any new frame.
   // This check accounts for a future change where the value of iteration_value->frames_count may potentially be greater than 0.
   // It's essential to validate that this increase doesn't surpass the maximum number of frames we can process.
   if (iteration_value->frames_count >= HTTP2_MAX_FRAMES_ITERATIONS) {
       return;
   }

    __u32 iteration = 0;
#pragma unroll(HTTP2_MAX_FRAMES_TO_FILTER)
    for (; iteration < HTTP2_MAX_FRAMES_TO_FILTER; ++iteration) {
        // Checking we can read HTTP2_FRAME_HEADER_SIZE from the skb.
        if (info->off + HTTP2_FRAME_HEADER_SIZE > info->len) {
            break;
        }

        read_into_user_buffer_http2_frame_header((char *)&current_frame, info->buffer_ptr + info->off);
        info->off += HTTP2_FRAME_HEADER_SIZE;
        if (!format_http2_frame_header(&current_frame)) {
            break;
        }

        // END_STREAM can appear only in Headers and Data frames.
        // Check out https://datatracker.ietf.org/doc/html/rfc7540#section-6.1 for data frame, and
        // https://datatracker.ietf.org/doc/html/rfc7540#section-6.2 for headers frame.
        is_headers_or_rst_frame = current_frame.type == kHeadersFrame || current_frame.type == kRSTStreamFrame;
        is_data_end_of_stream = ((current_frame.flags & HTTP2_END_OF_STREAM) == HTTP2_END_OF_STREAM) && (current_frame.type == kDataFrame);
        if (iteration_value->frames_count < HTTP2_MAX_FRAMES_ITERATIONS && (is_headers_or_rst_frame || is_data_end_of_stream)) {
            iteration_value->frames_array[iteration_value->frames_count].frame = current_frame;
            iteration_value->frames_array[iteration_value->frames_count].offset = info->off;
            iteration_value->frames_count++;
        }
        info->off += current_frame.length;

        // If we have found enough interesting frames, we can stop iterating.
        if (iteration_value->frames_count >= HTTP2_MAX_FRAMES_ITERATIONS) {
            break;
        }
    }

    // Checking we can read HTTP2_FRAME_HEADER_SIZE from the skb - if we can, update telemetry to indicate we have
    if ((iteration == HTTP2_MAX_FRAMES_TO_FILTER) && (info->off + HTTP2_FRAME_HEADER_SIZE <= info->len)) {
        __sync_fetch_and_add(&http2_tel->exceeding_max_frames_to_filter, 1);
    }

    if (iteration_value->frames_count == HTTP2_MAX_FRAMES_ITERATIONS) {
        __sync_fetch_and_add(&http2_tel->exceeding_max_interesting_frames, 1);
    }
}

SEC("uprobe/http2_tls_handle_first_frame")
int uprobe__http2_tls_handle_first_frame(struct pt_regs *ctx) {
    return 0;
}

SEC("uprobe/http2_tls_filter")
int uprobe__http2_tls_filter(struct pt_regs *ctx) {
    return 0;
}

SEC("uprobe/http2_tls_headers_parser")
int uprobe__http2_tls_headers_parser(struct pt_regs *ctx) {
    return 0;
}

SEC("uprobe/http2_tls_eos_parser")
int uprobe__http2_tls_eos_parser(struct pt_regs *ctx) {
    return 0;
}

SEC("uprobe/http2_tls_termination")
int uprobe__http2_tls_termination(struct pt_regs *ctx) {
    const __u32 zero = 0;

    tls_dispatcher_arguments_t *args = bpf_map_lookup_elem(&tls_dispatcher_arguments, &zero);
    if (args == NULL) {
        return 0;
    }

    terminated_http2_batch_enqueue(&args->tup);
    // Deleting the entry for the original tuple.
    bpf_map_delete_elem(&http2_dynamic_counter_table, &args->tup);
    // In case of local host, the protocol will be deleted for both (client->server) and (server->client),
    // so we won't reach for that path again in the code, so we're deleting the opposite side as well.
    flip_tuple(&args->tup);
    bpf_map_delete_elem(&http2_dynamic_counter_table, &args->tup);

    bpf_map_delete_elem(&tls_http2_iterations, &args->tup);

    return 0;
}
#endif
