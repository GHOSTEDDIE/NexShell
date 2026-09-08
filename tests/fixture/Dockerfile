FROM ubuntu:24.04
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends openssh-server openssh-sftp-server bash coreutils procps iproute2 util-linux vim-tiny tmux lrzsz nginx curl tar iputils-ping traceroute && rm -rf /var/lib/apt/lists/*
RUN ssh-keygen -A && mkdir -p /run/sshd /tmp/fixture && printf '%s\n' 'PermitRootLogin yes' 'PasswordAuthentication yes' >> /etc/ssh/sshd_config
COPY entrypoint.sh /entrypoint.sh
EXPOSE 22
ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
