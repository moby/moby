ALL_PAGES := $(wildcard *.*.md)

man_section = $(lastword $(subst ., ,$(1)))

$(foreach page,$(ALL_PAGES:.md=),$(eval all: man$(call man_section,$(page))/$(page)))

.PRECIOUS: man%
man%:
	mkdir -p $@

define MANPAGE_template
man$(1)/%.$(1): %.$(1).md man$(1) build/go-md2man
	build/go-md2man -in $$< -out $$@
endef

sections := $(sort $(foreach page,$(ALL_PAGES:.md=),$(call man_section,$(page))))
$(foreach sec,$(sections),$(eval $(call MANPAGE_template,$(sec))))

build/go-md2man: go.mod go.sum
	GO111MODULE=auto go build -o $@ github.com/cpuguy83/go-md2man/v2

.PHONY: clean
clean:
	rm -r man* build
