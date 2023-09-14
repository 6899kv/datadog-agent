#include "kconfig.h"
#include "offset-guess.h"
#include "bpf_tracing.h"
#include "bpf_endian.h"
#include "map-defs.h"

#include <net/net_namespace.h>
#include <net/sock.h>
#include <net/flow.h>
#include <uapi/linux/ptrace.h>
#include <uapi/linux/tcp.h>
#include <uapi/linux/ip.h>

// aligned_offset returns an offset that when added to
// p, would produce an address that is mod size (aligned).
//
// This function works in concert with the offset guessing
// code in pkg/network/tracer/offsetguess.go that will
// increment the returned here by 1 (thus yielding an offset
// that will not produce an aligned address anymore). When
// that offset is passed in here on subsequent calls, it
// has the affect of producing an offset that will move
// p to the next address mod size.
static __always_inline u64 aligned_offset(void *p, u64 offset, uintptr_t size) {
    u64 _p = (u64)p;
    _p += offset;
    // for a value of _p that is not mod size
    // we want to advance to the next _p that is
    // mod size
    _p = _p + size - 1 - (_p + size - 1) % size;
    return (char*)_p - (char*)p;
}

/* These maps are used to match the kprobe & kretprobe of connect for IPv6 */
/* This is a key/value store with the keys being a pid
 * and the values being a struct sock *.
 */
BPF_HASH_MAP(connectsock_ipv6, __u64, void*, 1024)

BPF_HASH_MAP(tracer_status, __u64, tracer_status_t, 1)
BPF_HASH_MAP(conntrack_status, __u64, conntrack_status_t, 1)

static __always_inline bool proc_t_comm_equals(proc_t a, proc_t b) {
    for (int i = 0; i < TASK_COMM_LEN; i++) {
        if (a.comm[i] != b.comm[i]) {
            return false;
        }
        // if chars equal but a NUL terminator, both strings equal
        if (!a.comm[i]) {
            break;
        }
    }
    return true;
}

static __always_inline bool check_family(struct sock* sk, tracer_status_t* status, u16 expected_family) {
    u16 family = 0;
    bpf_probe_read_kernel(&family, sizeof(u16), ((char*)sk) + status->offsets.family);
    return family == expected_family;
}

static __always_inline int guess_offsets(tracer_status_t* status, char* subject) {
    u64 zero = 0;

    if (status->state != STATE_CHECKING) {
        return 1;
    }

    // Only traffic for the expected process name. Extraneous connections from other processes must be ignored here.
    // Userland must take care to generate connections from the correct thread. In Golang, this can be achieved
    // with runtime.LockOSThread.
    proc_t proc = {};
    bpf_get_current_comm(&proc.comm, sizeof(proc.comm));

    if (!proc_t_comm_equals(status->proc, proc)) {
        return 0;
    }

    tracer_status_t new_status = {};
    // Copy values from status to new_status
    bpf_probe_read_kernel(&new_status, sizeof(tracer_status_t), status);
    new_status.state = STATE_CHECKED;
    new_status.err = 0;
    bpf_probe_read_kernel(&new_status.proc.comm, sizeof(proc.comm), proc.comm);

    possible_net_t* possible_skc_net = NULL;
    u32 possible_netns = 0;
    long ret;

    switch (status->what) {
    case GUESS_SADDR:
        new_status.offsets.saddr = aligned_offset(subject, status->offsets.saddr, SIZEOF_SADDR);
        bpf_probe_read_kernel(&new_status.values.saddr, sizeof(new_status.values.saddr), subject + new_status.offsets.saddr);
        break;
    case GUESS_DADDR:
        new_status.offsets.daddr = aligned_offset(subject, status->offsets.daddr, SIZEOF_DADDR);
        bpf_probe_read_kernel(&new_status.values.daddr, sizeof(new_status.values.daddr), subject + new_status.offsets.daddr);
        break;
    case GUESS_FAMILY:
        new_status.offsets.family = aligned_offset(subject, status->offsets.family, SIZEOF_FAMILY);
        bpf_probe_read_kernel(&new_status.values.family, sizeof(new_status.values.family), subject + new_status.offsets.family);
        break;
    case GUESS_SPORT:
        new_status.offsets.sport = aligned_offset(subject, status->offsets.sport, SIZEOF_SPORT);
        bpf_probe_read_kernel(&new_status.values.sport, sizeof(new_status.values.sport), subject + new_status.offsets.sport);
        new_status.values.sport = bpf_ntohs(new_status.values.sport);
        break;
    case GUESS_DPORT:
        new_status.offsets.dport = aligned_offset(subject, status->offsets.dport, SIZEOF_DPORT);
        bpf_probe_read_kernel(&new_status.values.dport, sizeof(new_status.values.dport), subject + new_status.offsets.dport);
        new_status.values.dport = bpf_ntohs(new_status.values.dport);
        break;
    case GUESS_SADDR_FL4:
        new_status.offsets.saddr_fl4 = aligned_offset(subject, status->offsets.saddr_fl4, SIZEOF_SADDR_FL4);
        bpf_probe_read_kernel(&new_status.values.saddr_fl4, sizeof(new_status.values.saddr_fl4), subject + new_status.offsets.saddr_fl4);
        break;
    case GUESS_DADDR_FL4:
        new_status.offsets.daddr_fl4 = aligned_offset(subject, status->offsets.daddr_fl4, SIZEOF_DADDR_FL4);
        bpf_probe_read_kernel(&new_status.values.daddr_fl4, sizeof(new_status.values.daddr_fl4), subject + new_status.offsets.daddr_fl4);
        break;
    case GUESS_SPORT_FL4:
        new_status.offsets.sport_fl4 = aligned_offset(subject, status->offsets.sport_fl4, SIZEOF_SPORT_FL4);
        bpf_probe_read_kernel(&new_status.values.sport_fl4, sizeof(new_status.values.sport_fl4), subject + new_status.offsets.sport_fl4);
        new_status.values.sport_fl4 = bpf_ntohs(new_status.values.sport_fl4);
        break;
    case GUESS_DPORT_FL4:
        new_status.offsets.dport_fl4 = aligned_offset(subject, status->offsets.dport_fl4, SIZEOF_DPORT_FL4);
        bpf_probe_read_kernel(&new_status.values.dport_fl4, sizeof(new_status.values.dport_fl4), subject + new_status.offsets.dport_fl4);
        new_status.values.dport_fl4 = bpf_ntohs(new_status.values.dport_fl4);
        break;
    case GUESS_SADDR_FL6:
        new_status.offsets.saddr_fl6 = aligned_offset(subject, status->offsets.saddr_fl6, SIZEOF_SADDR_FL6);
        bpf_probe_read_kernel(&new_status.values.saddr_fl6, sizeof(u32) * 4, subject + new_status.offsets.saddr_fl6);
        break;
    case GUESS_DADDR_FL6:
        new_status.offsets.daddr_fl6 = aligned_offset(subject, status->offsets.daddr_fl6, SIZEOF_DADDR_FL6);
        bpf_probe_read_kernel(&new_status.values.daddr_fl6, sizeof(u32) * 4, subject + new_status.offsets.daddr_fl6);
        break;
    case GUESS_SPORT_FL6:
        new_status.offsets.sport_fl6 = aligned_offset(subject, status->offsets.sport_fl6, SIZEOF_SPORT_FL6);
        bpf_probe_read_kernel(&new_status.values.sport_fl6, sizeof(new_status.values.sport_fl6), subject + new_status.offsets.sport_fl6);
        new_status.values.sport_fl6 = bpf_ntohs(new_status.values.sport_fl6);
        break;
    case GUESS_DPORT_FL6:
        new_status.offsets.dport_fl6 = aligned_offset(subject, status->offsets.dport_fl6, SIZEOF_DPORT_FL6);
        bpf_probe_read_kernel(&new_status.values.dport_fl6, sizeof(new_status.values.dport_fl6), subject + new_status.offsets.dport_fl6);
        new_status.values.dport_fl6 = bpf_ntohs(new_status.values.dport_fl6);
        break;
    case GUESS_NETNS:
        new_status.offsets.netns = aligned_offset(subject, status->offsets.netns, SIZEOF_NETNS);
        bpf_probe_read_kernel(&possible_skc_net, sizeof(possible_net_t*), subject + new_status.offsets.netns);
        if (!possible_skc_net) {
            new_status.err = 1;
            break;
        }
        // if we get a kernel fault, it means possible_skc_net
        // is an invalid pointer, signal an error so we can go
        // to the next offsets.netns
        new_status.offsets.ino = aligned_offset(subject, status->offsets.ino, SIZEOF_NETNS_INO);
        ret = bpf_probe_read_kernel(&possible_netns, sizeof(possible_netns), (char*)possible_skc_net + new_status.offsets.ino);
        if (ret == -EFAULT) {
            new_status.err = 1;
            break;
        }
        //log_debug("netns: off=%u ino=%u val=%u\n", status->offsets.netns, status->offsets.ino, possible_netns);
        new_status.values.netns = possible_netns;
        break;
    case GUESS_RTT:
        new_status.offsets.rtt = aligned_offset(subject, status->offsets.rtt, SIZEOF_RTT);
        bpf_probe_read_kernel(&new_status.values.rtt, sizeof(new_status.values.rtt), subject + new_status.offsets.rtt);
        // We know that these two fields are always next to each other, 4 bytes apart:
        // https://elixir.bootlin.com/linux/v4.6/source/include/linux/tcp.h#L232
        // rtt -> srtt_us
        // rtt_var -> mdev_us
        new_status.offsets.rtt_var = aligned_offset(subject, new_status.offsets.rtt + SIZEOF_RTT, SIZEOF_RTT_VAR);
        bpf_probe_read_kernel(&new_status.values.rtt_var, sizeof(new_status.values.rtt_var), subject + new_status.offsets.rtt_var);
        // For more information on the bit shift operations see:
        // https://elixir.bootlin.com/linux/v4.6/source/net/ipv4/tcp.c#L2686
        new_status.values.rtt = new_status.values.rtt >> 3;
        new_status.values.rtt_var = new_status.values.rtt_var >> 2;
        break;
    case GUESS_DADDR_IPV6:
        if (!check_family((struct sock*)subject, status, AF_INET6)) {
            break;
        }

        new_status.offsets.daddr_ipv6 = aligned_offset(subject, status->offsets.daddr_ipv6, SIZEOF_DADDR_IPV6);
        bpf_probe_read_kernel(new_status.values.daddr_ipv6, sizeof(u32) * 4, subject + new_status.offsets.daddr_ipv6);
        break;
    case GUESS_SOCKET_SK:
        // Note that in this line we're essentially dereferencing a pointer
        // subject initially points to a (struct socket*), and we're trying to guess the offset of
        // (struct socket*)->sk which points to a (struct sock*) object.
        new_status.offsets.socket_sk = aligned_offset(subject, status->offsets.socket_sk, SIZEOF_SOCKET_SK);
        bpf_probe_read_kernel(&subject, sizeof(subject), subject + new_status.offsets.socket_sk);
        bpf_probe_read_kernel(&new_status.values.sport_via_sk, sizeof(new_status.values.sport_via_sk), subject + new_status.offsets.sport);
        bpf_probe_read_kernel(&new_status.values.dport_via_sk, sizeof(new_status.values.dport_via_sk), subject + new_status.offsets.dport);
        new_status.values.sport_via_sk = bpf_ntohs(new_status.values.sport_via_sk);
        new_status.values.dport_via_sk = bpf_ntohs(new_status.values.dport_via_sk);
        break;
    case GUESS_SK_BUFF_SOCK:
        // Note that in this line we're essentially dereferencing a pointer
        // subject initially points to a (struct sk_buff*), and we're trying to guess the offset of
        // (struct sk_buff*)->sk which points to a (struct sock*) object.
        new_status.offsets.sk_buff_sock = aligned_offset(subject, status->offsets.sk_buff_sock, SIZEOF_SK_BUFF_SOCK);
        bpf_probe_read_kernel(&subject, sizeof(subject), subject + new_status.offsets.sk_buff_sock);
        bpf_probe_read_kernel(&new_status.values.sport_via_sk_via_sk_buff, sizeof(new_status.values.sport_via_sk_via_sk_buff), subject + new_status.offsets.sport);
        bpf_probe_read_kernel(&new_status.values.dport_via_sk_via_sk_buff, sizeof(new_status.values.dport_via_sk_via_sk_buff), subject + new_status.offsets.dport);
        new_status.values.sport_via_sk_via_sk_buff = bpf_ntohs(new_status.values.sport_via_sk_via_sk_buff);
        new_status.values.dport_via_sk_via_sk_buff = bpf_ntohs(new_status.values.dport_via_sk_via_sk_buff);
        break;
    case GUESS_SK_BUFF_TRANSPORT_HEADER:
        new_status.offsets.sk_buff_transport_header = aligned_offset(subject, status->offsets.sk_buff_transport_header, SIZEOF_SK_BUFF_TRANSPORT_HEADER);
        bpf_probe_read_kernel(&new_status.values.transport_header, sizeof(new_status.values.transport_header), subject + new_status.offsets.sk_buff_transport_header);
        bpf_probe_read_kernel(&new_status.values.network_header, sizeof(new_status.values.network_header), subject + new_status.offsets.sk_buff_transport_header + sizeof(__u16));
        bpf_probe_read_kernel(&new_status.values.mac_header, sizeof(new_status.values.mac_header), subject + new_status.offsets.sk_buff_transport_header + 2*sizeof(__u16));
        break;
    case GUESS_SK_BUFF_HEAD:
        // Loading the head field into `subject`.
        new_status.offsets.sk_buff_head = aligned_offset(subject, status->offsets.sk_buff_head, SIZEOF_SK_BUFF_HEAD);
        bpf_probe_read_kernel(&subject, sizeof(subject), subject + new_status.offsets.sk_buff_head);
        // Loading source and dest ports.
        // The ports are located in the transport section (subject + status->transport_header), if the traffic is udp or tcp
        // the source port is the first field in the struct (16 bits), and the dest is the second field (16 bits).
        bpf_probe_read_kernel(&new_status.values.sport_via_sk_buff, sizeof(new_status.values.sport_via_sk_buff), subject + status->values.transport_header);
        bpf_probe_read_kernel(&new_status.values.dport_via_sk_buff, sizeof(new_status.values.dport_via_sk_buff), subject + status->values.transport_header + sizeof(__u16));
        new_status.values.sport_via_sk_buff = bpf_ntohs(new_status.values.sport_via_sk_buff);
        new_status.values.dport_via_sk_buff = bpf_ntohs(new_status.values.dport_via_sk_buff);
        break;
    default:
        // not for us
        return 0;
    }

    bpf_map_update_elem(&tracer_status, &zero, &new_status, BPF_ANY);

    return 0;
}

static __always_inline bool is_sk_buff_event(__u64 what) {
    return what == GUESS_SK_BUFF_SOCK || what == GUESS_SK_BUFF_TRANSPORT_HEADER || what == GUESS_SK_BUFF_HEAD;
}

SEC("kprobe/ip_make_skb")
int kprobe__ip_make_skb(struct pt_regs* ctx) {
    u64 zero = 0;
    tracer_status_t* status = bpf_map_lookup_elem(&tracer_status, &zero);

    if (status == NULL || is_sk_buff_event(status->what)) {
        return 0;
    }

    struct flowi4* fl4 = (struct flowi4*)PT_REGS_PARM2(ctx);
    guess_offsets(status, (char*)fl4);
    return 0;
}

SEC("kprobe/ip6_make_skb")
int kprobe__ip6_make_skb(struct pt_regs* ctx) {
    u64 zero = 0;
    tracer_status_t* status = bpf_map_lookup_elem(&tracer_status, &zero);
    if (status == NULL || is_sk_buff_event(status->what)) {
        return 0;
    }
    struct flowi6* fl6 = (struct flowi6*)PT_REGS_PARM7(ctx);
    guess_offsets(status, (char*)fl6);
    return 0;
}

SEC("kprobe/ip6_make_skb")
int kprobe__ip6_make_skb__pre_4_7_0(struct pt_regs* ctx) {
    u64 zero = 0;
    tracer_status_t* status = bpf_map_lookup_elem(&tracer_status, &zero);
    if (status == NULL || is_sk_buff_event(status->what)) {
        return 0;
    }
    struct flowi6* fl6 = (struct flowi6*)PT_REGS_PARM9(ctx);
    guess_offsets(status, (char*)fl6);
    return 0;
}

/* Used exclusively for offset guessing */
SEC("kprobe/tcp_getsockopt")
int kprobe__tcp_getsockopt(struct pt_regs* ctx) {
    int level = (int)PT_REGS_PARM2(ctx);
    int optname = (int)PT_REGS_PARM3(ctx);
    if (level != SOL_TCP || optname != TCP_INFO) {
        return 0;
    }

    u64 zero = 0;
    tracer_status_t* status = bpf_map_lookup_elem(&tracer_status, &zero);
    if (status == NULL || status->what == GUESS_SOCKET_SK || is_sk_buff_event(status->what)) {
        return 0;
    }
    struct sock* sk = (struct sock*)PT_REGS_PARM1(ctx);
    status->tcp_info_kprobe_status = 1;
    guess_offsets(status, (char*)sk);

    return 0;
}

/* Used for offset guessing the struct socket->sk field */
SEC("kprobe/sock_common_getsockopt")
int kprobe__sock_common_getsockopt(struct pt_regs* ctx) {
    u64 zero = 0;
    tracer_status_t* status = bpf_map_lookup_elem(&tracer_status, &zero);
    if (status == NULL || status->what != GUESS_SOCKET_SK) {
        return 0;
    }

    struct socket* socket = (struct socket*)PT_REGS_PARM1(ctx);
    guess_offsets(status, (char*)socket);
    return 0;
}

// Used for offset guessing (see: pkg/ebpf/offsetguess.go)
SEC("kprobe/tcp_v6_connect")
int kprobe__tcp_v6_connect(struct pt_regs* ctx) {
    struct sock* sk;
    u64 pid = bpf_get_current_pid_tgid();

    sk = (struct sock*)PT_REGS_PARM1(ctx);

    bpf_map_update_elem(&connectsock_ipv6, &pid, &sk, BPF_ANY);

    return 0;
}

// Used for offset guessing (see: pkg/ebpf/offsetguess.go)
SEC("kretprobe/tcp_v6_connect")
int kretprobe__tcp_v6_connect(struct pt_regs* __attribute__((unused)) ctx) {
    u64 pid = bpf_get_current_pid_tgid();
    u64 zero = 0;
    struct sock** skpp;
    tracer_status_t* status;
    skpp = bpf_map_lookup_elem(&connectsock_ipv6, &pid);
    if (skpp == 0) {
        return 0; // missed entry
    }

    struct sock* skp = *skpp;
    bpf_map_delete_elem(&connectsock_ipv6, &pid);

    status = bpf_map_lookup_elem(&tracer_status, &zero);
    if (status == NULL || is_sk_buff_event(status->what)) {
        return 0;
    }
    // We should figure out offsets if they're not already figured out
    guess_offsets(status, (char*)skp);

    return 0;
}

struct net_dev_queue_ctx {
    u64 unused;
    void* skb;
};

SEC("tracepoint/net/net_dev_queue")
int tracepoint__net__net_dev_queue(struct net_dev_queue_ctx* ctx) {
    u64 zero = 0;
    tracer_status_t* status = bpf_map_lookup_elem(&tracer_status, &zero);
    // If we've triggered the hook and we are not under the context of guess offsets for GUESS_SK_BUFF_SOCK,
    // GUESS_SK_BUFF_TRANSPORT_HEADER, or GUESS_SK_BUFF_HEAD then we should do nothing in the hook.
    if (status == NULL || !is_sk_buff_event(status->what)) {
        return 0;
    }

    guess_offsets(status, ctx->skb);
    return 0;
}

static __always_inline int guess_conntrack_offsets(conntrack_status_t* status, char* subject) {
    u64 zero = 0;

    if (status->state != STATE_CHECKING) {
        return 1;
    }

    // Only traffic for the expected process name. Extraneous connections from other processes must be ignored here.
    // Userland must take care to generate connections from the correct thread. In Golang, this can be achieved
    // with runtime.LockOSThread.
    proc_t proc = {};
    bpf_get_current_comm(&proc.comm, sizeof(proc.comm));

    if (!proc_t_comm_equals(status->proc, proc)) {
        return 0;
    }

    conntrack_status_t new_status = {};
    // Copy values from status to new_status
    bpf_probe_read_kernel(&new_status, sizeof(conntrack_status_t), status);
    new_status.state = STATE_CHECKED;
    bpf_probe_read_kernel(&new_status.proc.comm, sizeof(proc.comm), proc.comm);

    possible_net_t* possible_ct_net = NULL;
    u32 possible_netns = 0;
    switch (status->what) {
    case GUESS_CT_TUPLE_ORIGIN:
        new_status.offsets.origin = aligned_offset(subject, status->offsets.origin, SIZEOF_CT_TUPLE_ORIGIN);
        bpf_probe_read_kernel(&new_status.values.saddr, sizeof(new_status.values.saddr), subject + new_status.offsets.origin);
        break;
    case GUESS_CT_TUPLE_REPLY:
        new_status.offsets.reply = aligned_offset(subject, status->offsets.reply, SIZEOF_CT_TUPLE_REPLY);
        bpf_probe_read_kernel(&new_status.values.daddr, sizeof(new_status.values.daddr), subject + new_status.offsets.reply);
        break;
    case GUESS_CT_STATUS:
        new_status.offsets.status = aligned_offset(subject, status->offsets.status, SIZEOF_CT_STATUS);
        bpf_probe_read_kernel(&new_status.values.status, sizeof(new_status.values.status), subject + new_status.offsets.status);
        break;
    case GUESS_CT_NET:
        new_status.offsets.netns = aligned_offset(subject, status->offsets.netns, SIZEOF_CT_NET);
        bpf_probe_read_kernel(&possible_ct_net, sizeof(possible_net_t*), subject + new_status.offsets.netns);
        bpf_probe_read_kernel(&possible_netns, sizeof(possible_netns), ((char*)possible_ct_net) + status->offsets.ino);
        new_status.values.netns = possible_netns;
        break;
    default:
        // not for us
        return 0;
    }

    bpf_map_update_elem(&conntrack_status, &zero, &new_status, BPF_ANY);

    return 0;
}

static __always_inline bool is_ct_event(u64 what) {
    switch (what) {
    case GUESS_CT_TUPLE_ORIGIN:
    case GUESS_CT_TUPLE_REPLY:
    case GUESS_CT_STATUS:
    case GUESS_CT_NET:
        return true;
    default:
        return false;
    }
}

SEC("kprobe/__nf_conntrack_hash_insert")
int kprobe___nf_conntrack_hash_insert(struct pt_regs* ctx) {
    u64 zero = 0;
    conntrack_status_t* status = bpf_map_lookup_elem(&conntrack_status, &zero);
    if (status == NULL || !is_ct_event(status->what)) {
        return 0;
    }

    void *ct = (void*)PT_REGS_PARM1(ctx);
    guess_conntrack_offsets(status, (char*)ct);
    return 0;
}


char _license[] SEC("license") = "GPL";
