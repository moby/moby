FROM splunk/splunk:latest

# Setup HEC settings
COPY http_inputs.conf /tmp/http_inputs.conf
COPY server.conf /tmp/server.conf
COPY indexes.conf /tmp/indexes.conf
RUN mkdir -p ${SPLUNK_BACKUP_DEFAULT_ETC}/etc/apps/splunk_httpinput/local \
	&& cp /tmp/http_inputs.conf ${SPLUNK_BACKUP_DEFAULT_ETC}/etc/apps/splunk_httpinput/local/inputs.conf \
	&& mkdir -p ${SPLUNK_BACKUP_DEFAULT_ETC}/etc/apps/search/local \
	&& cp /tmp/indexes.conf ${SPLUNK_BACKUP_DEFAULT_ETC}/etc/apps/search/local/indexes.conf \
	&& cat /tmp/server.conf >> ${SPLUNK_BACKUP_DEFAULT_ETC}/etc/system/local/server.conf \
	&& rm -f /tmp/http_inputs.conf \
	&& rm -f /tmp/server.conf \
	&& rm -f /tmp/indexes.conf