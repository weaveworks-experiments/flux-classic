FROM krallin/ubuntu-tini:trusty

ENTRYPOINT ["/usr/bin/tini", "-g", "--"]

RUN apt-get update && apt-get install -y --no-install-recommends \
                ruby2.0 ruby2.0-dev ruby-dev \
	&& rm -rf /var/lib/apt/lists/*

# Because https://bugs.launchpad.net/ubuntu/+source/ruby2.0/+bug/1310292
RUN for i in erb gem irb rake rdoc ri ruby testrb ; do \
        sudo ln -sf /usr/bin/${i}2.0 /usr/bin/${i} ; \
    done

RUN gem install bundler

WORKDIR /home/flux
COPY ./Gemfile* /home/flux/
RUN bundler install
