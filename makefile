TEST_FILE_DIR ?= test_files/
TEST_FILES = $(addprefix $(TEST_FILE_DIR),TEST_FILE1.txt TEST_FILE2.jpg TEST_FILE3.flp TEST_LARGE.bin)

build-client:
	cd client && npm run build

test: build-client $(TEST_FILES)
	cd test_client && npm run build
	cd server && \
		go test -coverprofile="coverage.out" && \
		go tool cover -html=coverage.out -o coverage.html \

test-stream: build-client $(TEST_FILES)
	cd test_stream && npm run build
	node test_stream/index.js --files $(TEST_FILES)

test-receive: build-client $(TEST_FILES)
	cd test_receiver && npm run build
	cd test_client && npm run build
	cd test_receiver/test_server && \
		go run main.go
