# Copyright The Kmesh Authors.

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
 
.PHONY: build docker-build docker-run clean

HUB ?= ghcr.io/kmesh-net
ifeq ($(HUB),)
  $(error "HUB cannot be empty")
endif

TARGET ?= kmesh-dns
ifeq ($(TARGET),)
  $(error "TARGET cannot be empty")
endif

TAG ?= $(shell git rev-parse --verify HEAD)
ifeq ($(TAG),)
  $(error "TAG cannot be empty")
endif

# Define architecture with defaults
GOOS ?= linux
GOARCH ?= amd64

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o out/$(TARGET)

.PHONY: docker
docker: build
	docker build -t $(HUB)/$(TARGET):$(TAG) .

docker.push: docker
	docker push $(HUB)/$(TARGET):$(TAG)
	
clean:
	rm -f $(TARGET)

