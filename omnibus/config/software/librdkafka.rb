# https://github.com/confluentinc/librdkafka#build-from-source
# https://github.com/Homebrew/homebrew-core/blob/35f8763a90eab4203deb3a6ee2503cf0ddfd8c84/Formula/librdkafka.rb#L32
# https://github.com/confluentinc/confluent-kafka-python/blob/master/tools/windows-install-librdkafka.bat

require './lib/autotools.rb'

name "librdkafka"
default_version "2.2.0"

dependency "cyrus-sasl"

source :url => "https://github.com/confluentinc/librdkafka/archive/refs/tags/v#{version}.tar.gz",
        :sha256 => "af9a820cbecbc64115629471df7c7cecd40403b6c34bfdbb9223152677a47226",
        :extract => :seven_zip

relative_path "librdkafka-#{version}"

build do

  license "BSD-style"
  license_file "https://raw.githubusercontent.com/confluentinc/librdkafka/master/LICENSE"

  build_with_autotools({
    :discard_default_opts => true,
    :prefix => "#{install_dir}/embedded",
    :configure_opts => [
      "--enable-shared",
      "--enable-sasl",
      "--disable-dependency-tracking",
    ],
  })

  delete "#{install_dir}/embedded/lib/librdkafka.a"
  delete "#{install_dir}/embedded/lib/librdkafka-static.a"

end
