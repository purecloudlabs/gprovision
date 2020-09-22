#docker build -t c6q .

FROM centos:6

RUN set -eux;                                                                               \
    yum install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-6.noarch.rpm;  \
    yum install -y python34 glibc-static make bison flex git glib2-devel libfdt-devel       \
            zlib-devel pcre-static libtool zlib-static clang xz libffi-devel gettext-devel

WORKDIR /root

COPY linux-4.19.16.tar.xz libcap.txz libcap-ng.txz \
     attr-2.4.48.tar.gz pixman-0.38.4.tar.gz glib-2.45.8.tar.xz \
     edk2-ovmf-2017_p20180211-bin.tar.xz qemu.txz \
     /root

#compile with clang, as the installed gcc lacks support for some things in the newer code
ARG CC=clang
ARG CPP="clang++ -E"
ARG CXX=clang++
ARG TERM=xterm-256color

#centos 6 uses 2.6.x kernel headers, far too old. but installing the headers
#wipes out scsi/sg.h which we need... so re-install glibc-headers after that
RUN set -eux;                                                       \
    tar xJf linux-4.19.16.tar.xz;                                   \
    cd linux-4.19.16;                                               \
    make -j$(($(nproc) + 1)) defconfig;                             \
    make -j$(($(nproc) + 1)) headers_install INSTALL_HDR_PATH=/usr; \
    rpm -e --nodeps glibc-headers;                                  \
    yum install -y glibc-headers

#pkgconfig - existing are in lib64, but new pkgs will install to lib
RUN set -eux;                                     \
    mkdir -p /usr/lib/pkgconfig;                  \
    mv /usr/lib64/pkgconfig/* /usr/lib/pkgconfig; \
    rm -rf /usr/lib64/pkgconfig;                  \
    ln -s /usr/lib/pkgconfig /usr/lib64/pkgconfig

#curl http://download-mirror.savannah.gnu.org/releases/attr/attr-2.4.48.tar.gz >attr-2.4.48.tar.gz
RUN set -eux;                                       \
    tar xzf attr-2.4.48.tar.gz;                     \
    mkdir attr-2.4.48/build;                        \
    cd attr-2.4.48/build;                           \
    ../configure --enable-static --disable-shared;  \
    make -j$(($(nproc) + 1));                       \
    make install

#git clone --depth 1 https://github.com/stevegrubb/libcap-ng.git
RUN set -eux;                    \
    tar xJf libcap-ng.txz;       \
    cd libcap-ng;                \
    ./autogen.sh;                \
    ./configure --prefix=/usr;   \
    make -j$(($(nproc) + 1));    \
    make install


#git clone --depth 1 https://git.kernel.org/pub/scm/libs/libcap/libcap.git
RUN set -eux;                 \
    tar xJf libcap.txz;       \
    cd libcap;                \
    make -j$(($(nproc) + 1)); \
    make install

#curl https://www.cairographics.org/releases/pixman-0.38.4.tar.gz >pixman-0.38.4.tar.gz
RUN set -eux;                       \
    tar xzf pixman-0.38.4.tar.gz;   \
    cd pixman-0.38.4;               \
    ./configure --prefix=/usr;      \
    make -j$(($(nproc) + 1));       \
    make install

#glib http://ftp.gnome.org/pub/gnome/sources/glib/2.45/glib-2.45.8.tar.xz
RUN set -eux;                                   \
    tar xJf glib-2.45.8.tar.xz;                 \
    cd glib-2.45.8;                             \
    ./configure --prefix=/usr --enable-static;  \
    make -j$(($(nproc) + 1));                   \
    make install

#finally... qemu
RUN set -eux;                                                      \
    tar xJf qemu.txz;                                              \
    mkdir qemu/build;                                              \
    cd qemu/build;                                                 \
    ../configure                                                   \
        --target-list=x86_64-softmmu                               \
        --audio-drv-list=                                          \
        --python=/usr/bin/python3                                  \
        --disable-bzip2                                            \
        --disable-curl                                             \
        --disable-curses                                           \
        --disable-docs                                             \
        --disable-gcrypt                                           \
        --disable-gnutls                                           \
        --disable-gtk                                              \
        --disable-guest-agent                                      \
        --disable-kvm                                              \
        --disable-libssh2                                          \
        --disable-libusb                                           \
        --disable-libxml2                                          \
        --disable-live-block-migration                             \
        --disable-lzo                                              \
        --disable-mpath                                            \
        --disable-nettle                                           \
        --disable-numa                                             \
        --disable-opengl                                           \
        --disable-rdma                                             \
        --disable-replication                                      \
        --disable-sdl                                              \
        --disable-snappy                                           \
        --disable-spice                                            \
        --disable-tpm                                              \
        --disable-usb-redir                                        \
        --disable-vhost-crypto                                     \
        --disable-vhost-net                                        \
        --disable-vhost-scsi                                       \
        --disable-vhost-user                                       \
        --disable-vhost-vsock                                      \
        --disable-virglrenderer                                    \
        --disable-vnc                                              \
        --disable-vnc-jpeg                                         \
        --disable-vnc-png                                          \
        --disable-vnc-sasl                                         \
        --disable-vxhs                                             \
        --disable-xen                                              \
        --disable-debug-mutex                                      \
        --disable-debug-info                                       \
        --disable-debug-tcg                                        \
        --enable-virtfs                                            \
        --static;                                                  \
    make -j$(($(nproc) + 1)) qemu-img;                             \
    make -j$(($(nproc) + 1)) -C x86_64-softmmu qemu-system-x86_64; \
    mv qemu-img x86_64-softmmu/qemu-system-x86_64 ../..;           \
    cd ../..;                                                      \
    strip qemu-system-x86_64 qemu-img

#bios files, part of qemu
RUN set -eux;                                           \
    mkdir pc-bios;                                      \
    for f in bios-256k.bin       efi-e1000.rom          \
             efi-virtio.rom      kvmvapic.bin           \
             linuxboot_dma.bin   vgabios-stdvga.bin; do \
        mv qemu/pc-bios/$f pc-bios/;                    \
    done

#uefi (ovmf) firmware
#building seems complicated. at least for now, download pre-compiled binaries
#https://dev.gentoo.org/~tamiko/distfiles/edk2-ovmf-2017_p20180211-bin.tar.xz
RUN set -eux;                                                                                       \
    sha1sum -c <<< "a4017add2a00c06c7cdaa21aade9daf9774db285  edk2-ovmf-2017_p20180211-bin.tar.xz"; \
    tar xJf edk2-ovmf-2017_p20180211-bin.tar.xz;                                                    \
    mkdir ovmf;                                                                                     \
    mv edk2-ovmf-2017_p20180211/Build/OvmfX64/RELEASE_GCC49/FV/*.fd ovmf/

RUN ls -ld pc-bios qemu-system-x86_64 qemu-img ovmf;                    \
    tar cJf qemu_bins.txz pc-bios qemu-system-x86_64 qemu-img ovmf;     \
    echo "All done! copy the tarball out with:";                        \
    echo -e "$(tput setaf 2)id=\$(docker create c6q)";                  \
    echo "docker cp \$id:/root/qemu_bins.txz qemu_bins.txz";            \
    echo -e "docker rm -v \$id$(tput sgr0)"
