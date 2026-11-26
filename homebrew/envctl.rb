class Envctl < Formula
  desc "Environment files from a template and SOPS/age-encrypted secrets"
  homepage "https://github.com/PillowPillow/envctl"
  version "@VERSION@"
  license "GPL-3.0-only"

  on_macos do
    on_arm do
      url "https://github.com/PillowPillow/envctl/releases/download/v#{version}/envctl_darwin_arm64.tar.gz"
      sha256 "@SHA256_DARWIN_ARM64@"
    end
    on_intel do
      url "https://github.com/PillowPillow/envctl/releases/download/v#{version}/envctl_darwin_amd64.tar.gz"
      sha256 "@SHA256_DARWIN_AMD64@"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/PillowPillow/envctl/releases/download/v#{version}/envctl_linux_arm64.tar.gz"
      sha256 "@SHA256_LINUX_ARM64@"
    end
    on_intel do
      url "https://github.com/PillowPillow/envctl/releases/download/v#{version}/envctl_linux_amd64.tar.gz"
      sha256 "@SHA256_LINUX_AMD64@"
    end
  end

  depends_on "age"
  depends_on "sops"

  def install
    bin.install "envctl"
    generate_completions_from_executable(bin/"envctl", "completion")
  end

  test do
    assert_match "envctl #{version}", shell_output("#{bin}/envctl version")
  end
end
