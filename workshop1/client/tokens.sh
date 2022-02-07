#! /bin/bash

curl -X POST -v localhost:8080/tokens -d '{"size": 10}'
curl -X PUT -v localhost:8080/tokens -d '{"size": 10}'
