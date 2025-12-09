FROM debian:bookworm-slim

ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update -qq && \
    apt-get install -qq --no-install-recommends \
        squashfs-tools \
        cryptsetup-bin \
        qemu-utils \
        fakeroot \
    && apt-get clean && \
    rm -rf /var/lib/apt/lists/*

VOLUME /src
WORKDIR /src
