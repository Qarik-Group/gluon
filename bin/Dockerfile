FROM ubuntu:20.04

ARG BUILD_DATE
ARG VCS_REF
LABEL maintainer="James Hunt <images@huntprod.com>" \
      summary="Run Gluon in a Container" \
      org.label-schema.build-date=$BUILD_DATE \
      org.label-schema.vcs-url="https://github.com/starkandwayne/gluon.git" \
      org.label-schema.vcs-ref=$VCS_REF \
      org.label-schema.schema-version="1.0.0"

COPY gluon /usr/bin/gluon
RUN chmod 0755 /usr/bin/gluon

ENTRYPOINT ["gluon"]
