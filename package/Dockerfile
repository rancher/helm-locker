FROM registry.suse.com/bci/golang:1.22 as builder
RUN zypper -n install git vim less file curl wget
WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build

FROM registry.suse.com/bci/bci-micro:15.5
RUN echo 'helmlocker:x:1000:1000::/home/helmlocker:/bin/bash' >> /etc/passwd && \
    echo 'helmlocker:x:1000:' >> /etc/group && \
    mkdir /home/helmlocker && \
    chown -R helmlocker:helmlocker /home/helmlocker
COPY --from=builder /usr/src/app/bin/helm-locker /usr/bin/
USER helmlocker
CMD ["helm-locker"]
