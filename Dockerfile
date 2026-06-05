FROM gcr.io/distroless/static-debian12:nonroot

COPY xsmem /usr/local/bin/xsmem

ENTRYPOINT ["xsmem"]
