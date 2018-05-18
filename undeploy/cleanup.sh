#!/bin/bash

# The following commands will undo what you created running:
# kubectl create -f deploy/1.8+/
set -x
kubectl delete clusterrolebinding metrics-server:system:auth-delegator
kubectl -n kube-system delete rolebinding metrics-server-auth-reader
kubectl -n kube-system delete APIService v1beta1.metrics.k8s.io
kubectl -n kube-system delete serviceaccount metrics-server
kubectl -n kube-system delete deployment metrics-server
kubectl -n kube-system delete service metrics-server
kubectl delete clusterrole system:metrics-server
kubectl delete clusterrolebinding system:metrics-server
