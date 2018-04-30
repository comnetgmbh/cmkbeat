BEAT_NAME=cmkbeat
BEATDIR=github.com/jeremyweader/cmkbeat
ES_BEATS?=./vendor/github.com/elastic/beats
GOPACKAGES=$(shell glide novendor)
PREFIX?=.

# Path to the libbeat Makefile
-include $(ES_BEATS)/libbeat/scripts/Makefile
.PHONY: deps
deps:
	glide up
	
.PHONY: config
config:
	echo "Update config file"
	-rm -f ${BEAT_NAME}.yml
	cat etc/beat.yml etc/config.yml | sed -e "s/beatname/${BEAT_NAME}/g" > ${BEAT_NAME}.yml
	-rm -f ${BEAT_NAME}.full.yml
	cat etc/beat.yml etc/config.full.yml | sed -e "s/beatname/${BEAT_NAME}/g" > ${BEAT_NAME}.full.yml

	# Update doc
	python ${ES_BEATS}/libbeat/scripts/generate_fields_docs.py $(PWD) ${BEAT_NAME} ${ES_BEATS}

	# Generate index templates
	python ${ES_BEATS}/libbeat/scripts/generate_template.py $(PWD) ${BEAT_NAME} ${ES_BEATS}
	python ${ES_BEATS}/libbeat/scripts/generate_template.py --es2x $(PWD) ${BEAT_NAME} ${ES_BEATS}

	# Update docs version
	cp ${ES_BEATS}/libbeat/docs/version.asciidoc docs/version.asciidoc

	# Generate index-pattern
	echo "Generate index pattern"
	-rm -f $(PWD)/etc/kibana/index-pattern/${BEAT_NAME}.json
	mkdir -p $(PWD)/etc/kibana/index-pattern
	python ${ES_BEATS}/libbeat/scripts/generate_index_pattern.py --index ${BEAT_NAME}-* --libbeat ${ES_BEATS}/libbeat --beat $(PWD)


.PHONY: install
install:
	mkdir -p /etc/$(BEAT_NAME)
	mkdir -p /usr/share/$(BEAT_NAME)/bin
	mkdir -p /var/lib/$(BEAT_NAME)
	mkdir -p /var/log/$(BEAT_NAME)
	cp $(BEAT_NAME) /usr/share/$(BEAT_NAME)/bin/.
	cp *.yml /etc/$(BEAT_NAME)/.
	cp *.json /etc/$(BEAT_NAME)/.
	cp system/$(BEAT_NAME).service /usr/lib/systemd/system/.
	systemctl enable $(BEAT_NAME).service

.PHONY: uninstall
uninstall:
	systemctl disable $(BEAT_NAME).service
	rm /usr/lib/systemd/system/$(BEAT_NAME).service
	rm -rf /var/log/$(BEAT_NAME)
	rm -rf /var/lib/$(BEAT_NAME)
	rm -rf /usr/share/$(BEAT_NAME)
	rm -rf /etc/$(BEAT_NAME)
	
.PHONY: all
all:
	make deps
	make cmkbeat
