clean:
	rm -rf doc

deploy:
	ruby deploy.rb

node_modules:
	npm install apidoc

doc:
	node_modules/.bin/apidoc \
		--debug \
		-e node_modules \
		-e vendor \
		-e doc \
		-o doc

test:
	go test -v ./...

.PHONY: doc deploy
