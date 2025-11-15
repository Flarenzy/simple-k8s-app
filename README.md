# Simple IPAM application
The point of this repo is to learn on how to deploy a application on a minikube cluster.
The idea is to have a Kubernetes ingress, a NGINX or similar proxy for statically serviring the frontend, golang API for all CRUD operations, Keycloak for authorization and authentification and Postgres as a persistant data store.
Database migrations will be handeled by `goose` and `sqlc` will be used for type safe queries.
