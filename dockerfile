FROM debian:bullseye-slim
ENV DEBIAN_FRONTEND noninteractive
RUN apt update \
    && apt install ipmitool sysstat mount -y \
    && apt clean