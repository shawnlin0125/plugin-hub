.PHONY: build run docker clean deploy

# Local build
build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o plugin-hub .
	@ls -lh plugin-hub

# Local run
run: build
	./plugin-hub -config config/plugins.yaml

# Docker build
docker:
	docker build -t plugin-hub:go-latest .

# Docker run
docker-run: docker
	docker run --rm -p 8000:8000 plugin-hub:go-latest

# Deploy to k3s
deploy:
	kubectl apply -f k8s/deploy.yaml

# Clean
clean:
	rm -f plugin-hub
