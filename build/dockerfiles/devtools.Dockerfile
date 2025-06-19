# Copyright (c) 2021-2024 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation
#
FROM quay.io/devfile/universal-developer-image:latest

USER root

# Install gettext
RUN dnf install -y gettext

# Install the Operator SDK
ENV OPERATOR_SDK_VERSION="v0.17.2"
ENV OPERATOR_SDK_DL_URL=https://github.com/operator-framework/operator-sdk/releases/download/${OPERATOR_SDK_VERSION}
RUN curl -sSLO ${OPERATOR_SDK_DL_URL}/operator-sdk_linux_amd64 && \
    chmod +x operator-sdk_linux_amd64 && \
    mv operator-sdk_linux_amd64 /usr/local/bin/operator-sdk

# Install opm CLI
ENV OPM_VERSION="v1.13.1"
RUN curl -sSLO https://github.com/operator-framework/operator-registry/releases/download/${OPM_VERSION}/linux-amd64-opm && \
    chmod +x linux-amd64-opm && \
    mv linux-amd64-opm /usr/local/bin/opm

USER 1001