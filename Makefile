dev:
	go build main.go
	./main

commit:
	rm -rf main tmp
	git add .
	git commit