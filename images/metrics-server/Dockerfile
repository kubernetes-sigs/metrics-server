#
# This is the OpenShift Origin Metrics API Server.
#
# The standard name for this image is openshift/origin-metrics-server
#
FROM openshift/origin-base

RUN INSTALL_PKGS="origin-metrics-server" && \
    yum --enablerepo=origin-local-release install -y ${INSTALL_PKGS} && \
    rpm -V ${INSTALL_PKGS} && \
    yum clean all

LABEL io.k8s.display-name="OpenShift Container Platform Metrics Server" \
      io.k8s.description="This is a component of OpenShift Container Platform and exposes the cluster resources metrics API." \
      io.openshift.tags="openshift,metrics"

# The metrics-server doesn't require a root user.
USER 1001

CMD /usr/bin/metrics-server
