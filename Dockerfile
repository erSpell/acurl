# Runtime image for acurl. goreleaser builds the binaries and passes them in
# per platform.
FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/acurl /usr/local/bin/acurl
ENTRYPOINT ["/usr/local/bin/acurl"]
