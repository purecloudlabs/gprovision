
override THIS_FILE := $(abspath $(lastword $(MAKEFILE_LIST)))
override srcdir := $(dir $(THIS_FILE))

# use BUILDROOT v 2017.02

ifeq ($(BUILDROOT),)
  $(error "BUILDROOT must be set!")
else

default all: $(srcdir)/combined.cpio

.PHONY: br32 br32-show-tgts br32-uninst br64 br64-show-tgts br64-uninst

$(srcdir)/out32 $(srcdir)/out64 $(srcdir)/dl:
	mkdir -p $@

root64 := $(srcdir)/out64/images/rootfs.cpio
root32 := $(srcdir)/out32/images/rootfs.cpio
$(srcdir)/combined.cpio: $(root32) $(root64) | $(srcdir)/dl
	@cat $(srcdir)/out64/images/rootfs.cpio $(srcdir)/out32/images/rootfs.cpio >$(srcdir)/combined.cpio

common := BR2_EXTERNAL=$(srcdir) \
          BR2_JLEVEL=$(shell nproc) \
          BR2_DL_DIR=$(srcdir)/dl \
          BR2_GLOBAL_PATCH_DIR=$(srcdir)/patch

############## 32 bit ##############
cfg32 := $(srcdir)/configs/purecloudedge32_defconfig

# fakeroot script runs later than post_build script would, after
# everything that cares about the files we want to delete.
args32 := $(common) \
          BR2_CONFIG=$(cfg32) \
          BASE_DIR=$(srcdir)/out32 \
          BR2_ROOTFS_POST_FAKEROOT_SCRIPT=$(srcdir)/board/genesys/purecloudedge32/clean_fs.sh \
          BR2_ROOTFS_DEVICE_TABLE=$(srcdir)/emptyfile.txt \
          BR2_ROOTFS_STATIC_DEVICE_TABLE=$(srcdir)/emptyfile.txt

# if T is set, make that target. e.g. 'make br32 T=xconfig'
br32: $(root32)
$(root32): $(cfg32) | $(srcdir)/out32
	@$(MAKE) -C $(BUILDROOT) $(T) $(args32)
br32-show-tgts:
	@$(MAKE) -C $(BUILDROOT) show-targets $(args32)

# doesn't actually uninstall anything, just makes buildroot think they aren't installed
br32-uninst:
	@$(foreach i,$(shell ls $(srcdir)/out32/build/*/.stamp_target_installed | egrep -v 'host|toolchain'),rm $(i);)
rm32: | br32-uninst
	@rm -fr $(srcdir)/out32/target $(srcdir)/out32/images/*

############## 64 bit ##############
cfg64 := $(srcdir)/configs/purecloudedge64_defconfig

args64 := $(common) \
          BR2_CONFIG=$(cfg64) \
          BASE_DIR=$(srcdir)/out64 \
          BR2_ROOTFS_POST_FAKEROOT_SCRIPT=$(srcdir)/board/genesys/purecloudedge64/clean_fs.sh \
          BR2_ROOTFS_OVERLAY=$(srcdir)/board/genesys/purecloudedge64/overlay/

br64: $(root64)
# T below allows for things like `make br64 T=menuconfig`, building targets other than the default.
$(root64): $(cfg64) | $(srcdir)/out64
	@$(MAKE) -C $(BUILDROOT) $(T) $(args64)

br64-show-tgts:
	@$(MAKE) -C $(BUILDROOT) show-targets $(args64)

# doesn't actually uninstall anything, just makes buildroot think they aren't installed
br64-uninst:
	@$(foreach i,$(shell ls $(srcdir)/out64/build/*/.stamp_target_installed | egrep -v 'host|toolchain'),rm $(i);)
rm64: | br64-uninst
	@rm -fr $(srcdir)/out64/target $(srcdir)/out64/images/*

endif
