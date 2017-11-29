.DEFAULT_GOAL := all
PROJECT_ROOT := src/golang

NAME := spotitube
GOARCH := amd64
VERSION := $(shell awk -F'= ' '/VERSION / {print $$2}' src/system/constants.go)

include Makefile.gobuild
include Makefile.gobuild

BINARIES = \
	main

_PKGS = \
	logger \
	spotify \
	system \
	track \
	youtube

_CHECK_DEPENDENCIES = $(addsuffix .dependency,$(_DEPENDENCIES))

_CHECK_COMPLIANCE = $(addsuffix .compliant,$(_PKGS))

BINS = $(addsuffix .statbin,$(BINARIES))

dependencies:
	@ ( \
		regex_domain='(([a-zA-Z](-?[a-zA-Z0-9])*)\.)*[a-zA-Z](-?[a-zA-Z0-9])+\.[a-zA-Z]{2,}'; \
		find src -type f  \
			| egrep -v 'src\/'$$regex_domain'' \
			| xargs egrep '\"'$$regex_domain'\/.*\/.*\"' \
			| awk '{ print $$NF }' | grep -v ^$$ | sort -u | sed 's/"//g' | while read dependency; do \
			if [ ! -d $(CUR_DIR)/src/$$dependency ]; then \
				echo "Fetching $$dependency dependency"; \
			fi; \
			GOPATH=$(CUR_DIR)/ go get $$dependency || exit 1; \
		done; \
	);

compliant: $(_CHECK_COMPLIANCE)

install: $(BINS)
	@ ( \
		test -d $(DESTDIR)/usr/local/bin || install -D -d -m 00755 $(DESTDIR)/usr/local/bin; \
		install -m 00755 bin/* $(DESTDIR)/usr/local/bin/; \
	);

x86: GOARCH=386
x86: all

x64: all

release: packages unpackage

all: dependencies $(BINS)
