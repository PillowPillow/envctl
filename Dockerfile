FROM alpine:3.24

ARG SOPS_VERSION=3.13.3
ARG TARGETARCH

# Install age + certs
RUN apk add --no-cache age ca-certificates

# Install sops for the target architecture
RUN wget -qO /usr/local/bin/sops \
    "https://github.com/getsops/sops/releases/download/v${SOPS_VERSION}/sops-v${SOPS_VERSION}.linux.${TARGETARCH}" && \
    chmod +x /usr/local/bin/sops

# Copy the pre-built envctl binary for the target architecture
COPY bin/linux-${TARGETARCH}/envctl /usr/local/bin/envctl
RUN chmod +x /usr/local/bin/envctl

WORKDIR /workspace
ENTRYPOINT ["envctl"]
