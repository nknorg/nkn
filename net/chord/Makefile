
build:
	go build

test:
	go test .

cov:
	gocov test github.com/armon/go-chord | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

