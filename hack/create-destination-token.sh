#!/bin/bash
SOURCE_CLUSTER=$1
DESTINATION_CLUSTER=$2
NAMESPACE=$3
SERVICE_ACCOUNT_NAME=$4

kubectl config use-context $DESTINATION_CLUSTER
kubectl create namespace $NAMESPACE --ignore-already-exists
kubectl create serviceaccount $SERVICE_ACCOUNT_NAME -n $NAMESPACE


cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: $SERVICE_ACCOUNT_NAME-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: $SERVICE_ACCOUNT_NAME
  namespace: $NAMESPACE
EOF

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: $SERVICE_ACCOUNT_NAME-secret
  namespace: $NAMESPACE
  annotations:
    kubernetes.io/service-account.name: $SERVICE_ACCOUNT_NAME
type: kubernetes.io/service-account-token
EOF

# Wait for the secret to be created
echo "Waiting for the secret to be created..."
sleep 3
TOKEN=$(kubectl get secret -n $NAMESPACE $SERVICE_ACCOUNT_NAME-secret -oyaml | yq .data.token | base64 --decode)

kubectl config use-context
if kubectl get namespace application-rbac-validator-system; then
  echo "Namespace application-rbac-validator-system already exists."
else
  kubectl create namespace application-rbac-validator-system
fi
if kubectl get configmap -n application-rbac-validator-system application-rbac-validator-cluster-tokens; then
  echo "Configmap application-rbac-validator-cluster-tokens already exists."
else
  kubectl create configmap -n application-rbac-validator-system application-rbac-validator-cluster-tokens
fi
kubectl patch configmap -n application-rbac-validator-system application-rbac-validator-cluster-tokens --patch "{\"data\":{ \"${DESTINATION_CLUSTER}-token\": \"${TOKEN}\" }}" --type=merge
echo "Token for service account $SERVICE_ACCOUNT_NAME in cluster $DESTINATION_CLUSTER created and added to configmap."
echo "Token is $TOKEN"

