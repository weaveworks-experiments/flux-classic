$(call image_stamp,site): site/Gemfile site/Gemfile.lock

.PHONY: run-site
run-site: $(call image_stamp,site)
	docker run --rm -v $$PWD/site:/site \
		-v $$PWD/docker/build-wrapper.sh:/build-wrapper.sh \
		--workdir=/site -p 4000:4000 \
		$(call docker_tag,site) sh /build-wrapper.sh "bundler install && bundle exec jekyll serve --host 0.0.0.0"
