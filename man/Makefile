prefix = /usr/local
mandir = $(prefix)/man
INSTALL = install
INSTALL_DATA = ${INSTALL} -m 644

# By default, the man pages are generated using a copy of go-md2man built from
# the vendored sources in this directory. This behavior can be overridden by
# setting the GO_MD2MAN variable to the name/path of an existing go-md2man
# binary.
GO_MD2MAN ?= .build/go-md2man

ALL_PAGES := $(wildcard *.*.md)

# Determine which manual sections we are generating pages for
# by isolating the last part of the filename before the extension
# and eliminating duplicates.
man_section = $(lastword $(subst ., ,$(1)))
sections := $(sort $(foreach page,$(ALL_PAGES:.md=),$(call man_section,$(page))))

# Dynamically generate pattern rules for each manual section
# so make knows how to build a target like man8/dockerd.8.
define MANPAGE_template
man$(1)/%.$(1): %.$(1).md $(if $(findstring file,$(origin GO_MD2MAN)),$(GO_MD2MAN)) | man$(1)
	$(GO_MD2MAN) -in $$< -out $$@
endef
$(foreach sec,$(sections),$(eval $(call MANPAGE_template,$(sec))))

# Default target: build all man pages.
all: $(foreach page,$(ALL_PAGES:.md=),man$(call man_section,$(page))/$(page))

# Target for creating the man{1..8} directories as needed.
.PRECIOUS: man%
man%:
	-mkdir $@

.PHONY: install
install: all
	for sec in $(sections); do \
		$(INSTALL_DATA) man$$sec/* $(DESTDIR)$(mandir)/man$$sec; \
	done

.build/go-md2man: go.mod go.sum
	GO111MODULE=auto go build -o $@ github.com/cpuguy83/go-md2man/v2

.PHONY: clean
clean:
	rm -r man* .build
