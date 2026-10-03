FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6

# Bump SOPS_VERSION together with both checksums (sops-v<version>.checksums.txt).
ARG SOPS_VERSION=3.13.3
ARG SOPS_SHA256_AMD64=e5bec3346a873ae91d871550f3e698c1aad962aff462a080e40f25fde17fef6b
ARG SOPS_SHA256_ARM64=53b0abacd38ef1b12a66d6c100956691b9cefce018d91f81e73ddf7438b94d77
ARG TARGETARCH

# Install age + certs
RUN apk add --no-cache age ca-certificates

# Install sops for the target architecture
RUN case "${TARGETARCH}" in \
      amd64) sha="${SOPS_SHA256_AMD64}" ;; \
      arm64) sha="${SOPS_SHA256_ARM64}" ;; \
      *) echo "unsupported arch: ${TARGETARCH}" >&2; exit 1 ;; \
    esac && \
    wget -qO /usr/local/bin/sops \
    "https://github.com/getsops/sops/releases/download/v${SOPS_VERSION}/sops-v${SOPS_VERSION}.linux.${TARGETARCH}" && \
    echo "${sha}  /usr/local/bin/sops" | sha256sum -c - && \
    chmod +x /usr/local/bin/sops

# Copy the pre-built envctl binary for the target architecture
COPY bin/linux-${TARGETARCH}/envctl /usr/local/bin/envctl
RUN chmod +x /usr/local/bin/envctl

WORKDIR /workspace
ENTRYPOINT ["envctl"]
