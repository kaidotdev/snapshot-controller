FROM ghcr.io/kaidotdev/snapshot-controller/ghelpers/ghadapter:main AS ghadapter

FROM ghcr.io/kaidotdev/snapshot-controller/snapshot-capture:main

COPY --link --from=ghadapter /usr/local/bin/ghadapter /usr/local/bin/ghadapter

USER 1001

ENTRYPOINT ["/usr/local/bin/ghadapter", "/usr/local/bin/snapshot-capture"]
