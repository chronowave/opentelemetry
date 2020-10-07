FROM grafana/grafana:7.2.0

COPY grafana.ini /etc/grafana/
COPY ./dist /var/lib/grafana/plugins/
COPY custom.yaml /etc/grafana/provisioning/datasources/
