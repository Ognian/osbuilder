ARG ELEMENTAL_CLI_VERSION=0.20221107.1
ARG LEAP_VERSION=15.4
ARG LUET_VERSION=0.33.0
FROM quay.io/kairos/packages:elemental-cli-system-$ELEMENTAL_CLI_VERSION AS elemental
FROM quay.io/luet/base:$LUET_VERSION AS luet

### TODO: Replace those naked Dockerfiles copies with luet install so we can keep track of all versioning with 1 repository tag
### 1) Add the kairos repository with a reference
### 2) populate folders accordingly

## amd64 Live CD artifacts
FROM quay.io/kairos/packages:grub2-livecd-0.0.4 AS grub2
FROM quay.io/kairos/packages:grub2-efi-image-livecd-0.0.4 AS efi

## RPI64

## Firmware is in the amd64 repo (noarch)
FROM quay.io/kairos/packages:u-boot-rpi64-firmware-2021.01-5.1 AS rpi-u-boot
FROM quay.io/kairos/packages:raspberrypi-firmware-firmware-2021.03.10-2.1 AS rpi-firmware
FROM quay.io/kairos/packages:raspberrypi-firmware-config-firmware-2021.03.10-2.1 AS rpi-firmware-config
FROM quay.io/kairos/packages:raspberrypi-firmware-dt-firmware-2021.03.15-2.1 AS rpi-firmware-dt

## PineBook64 Pro
FROM quay.io/kairos/packages:u-boot-rockchip-arm-vendor-blob-0.1 AS pinebook-u-boot

## Generic ARM artifacts
FROM quay.io/kairos/packages-arm64:grub-efi-static-0.1  AS grub-efi
FROM quay.io/kairos/packages-arm64:grub-config-static-0.1  AS grub-config
FROM quay.io/kairos/packages-arm64:grub-artifacts-static-0.1   AS grub-artifacts

 ## RAW images
FROM quay.io/kairos/packages:grub-efi-static-0.1  AS grub-raw-efi
FROM quay.io/kairos/packages:grub-config-static-0.1  AS grub-raw-config
FROM quay.io/kairos/packages:grub-artifacts-static-0.1   AS grub-raw-artifacts

FROM opensuse/leap:$LEAP_VERSION
COPY --from=elemental /usr/bin/elemental /usr/bin/elemental
COPY --from=luet /usr/bin/luet /usr/bin/luet

# x86_64 ISOs
COPY --from=grub2 / /grub2
COPY --from=efi / /efi

# RAW images
COPY --from=grub-raw-efi / /raw/grub
COPY --from=grub-raw-config / /raw/grubconfig
COPY --from=grub-raw-artifacts / /raw/grubartifacts

# RPI64
COPY --from=rpi-u-boot / /rpi/u-boot
COPY --from=rpi-firmware / /rpi/rpi-firmware
COPY --from=rpi-firmware-config / /rpi/rpi-firmware-config
COPY --from=rpi-firmware-dt / /rpi/rpi-firmware-dt

# Pinebook
COPY --from=pinebook-u-boot / /pinebookpro/u-boot

# Generic
COPY --from=grub-efi / /arm/grub/efi
COPY --from=grub-config / /arm/grub/config
COPY --from=grub-artifacts / /arm/grub/artifacts

RUN zypper ref && zypper dup -y

## ISO Build depedencies
RUN zypper ref && zypper in -y xfsprogs parted util-linux-systemd e2fsprogs curl util-linux udev rsync grub2 dosfstools grub2-x86_64-efi squashfs mtools xorriso lvm2
RUN mkdir /config

# Arm image build deps
RUN zypper in -y jq docker git curl gptfdisk kpartx sudo
# Netboot
RUN zypper in -y cdrtools
# cloud images
RUN zypper in -y bc qemu-tools

# ISO build config
COPY ./config.yaml /config/manifest.yaml
COPY ./entrypoint.sh /entrypoint.sh
COPY ./add-cloud-init.sh /add-cloud-init.sh

# ARM helpers
COPY ./build-arm-image.sh /build-arm-image.sh
COPY ./arm /arm

# RAW images helpers
COPY ./gce.sh /gce.sh
COPY ./raw-images.sh /raw-images.sh
COPY ./azure.sh /azure.sh
COPY ./netboot.sh /netboot.sh

COPY defaults.yaml /defaults.yaml


ENTRYPOINT [ "/entrypoint.sh" ]
