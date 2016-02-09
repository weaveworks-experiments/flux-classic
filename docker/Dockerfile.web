FROM scratch

WORKDIR /home/flux
ENV HTTP_PORT=7070
EXPOSE 7070
COPY ./web /home/flux/web
COPY ./index.html /home/flux/
ADD ./assets.tar /home/flux/assets/
ENTRYPOINT ["/home/flux/web"]
