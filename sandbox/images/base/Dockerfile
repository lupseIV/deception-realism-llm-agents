# Agent container for environment realism "low": a Debian system with the
# packages of the aws-developer persona. The sandbox user is created at start
# by boot-init from the run's bundle.
FROM debian:bookworm-slim

RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      awscli bash ca-certificates curl git jq less openssh-client procps python3 vim-tiny \
 && rm -rf /var/lib/apt/lists/*

COPY boot-init /usr/local/sbin/boot-init
RUN chmod 755 /usr/local/sbin/boot-init

CMD ["sleep", "infinity"]
