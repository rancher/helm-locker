FROM registry.suse.com/bci/bci-micro:15.3
RUN echo 'helmlocker:x:1000:1000::/home/helmlocker:/bin/bash' >> /etc/passwd && \
    echo 'helmlocker:x:1000:' >> /etc/group && \
    mkdir /home/helmlocker && \
    chown -R helmlocker:helmlocker /home/helmlocker
COPY bin/helm-locker /usr/bin/
USER helmlocker
CMD ["helm-locker"]
