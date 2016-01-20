FROM node:4.2.2
ENV NPM_CONFIG_LOGLEVEL warn
WORKDIR /webbuild
ENV PATH /webbuild/node_modules/.bin:$PATH
COPY ./package.json ./
RUN npm install --no-optional
