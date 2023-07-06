FROM debian:bullseye-slim
ENV DEBIAN_FRONTEND noninteractive
RUN apt update \
    && apt install sysstat mount -y \
    && apt install ipmitool -y \
    && apt clean
COPY collector /app/collector
CMD [ "/app/collector" ]
