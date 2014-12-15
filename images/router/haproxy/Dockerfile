#
# This is the HAProxy router for OpenShift Origin.
#
# The standard name for this image is openshift/origin-haproxy-router
#
FROM openshift/origin-haproxy-router-base

ADD conf/ /var/lib/haproxy/conf/
ADD reload-haproxy /var/lib/haproxy/reload-haproxy
ADD bin/openshift /usr/bin/openshift
RUN ln -s /usr/bin/openshift /usr/bin/openshift-router

EXPOSE 80
ENTRYPOINT ["/usr/bin/openshift-router", "--template=/var/lib/haproxy/conf/haproxy-config.template", "--reload=/var/lib/haproxy/reload-haproxy"]
