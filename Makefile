.PHONY: shaders

all:

shaders:
	glslc -o tri.vert.spv tri.vert
	glslc -o tri.frag.spv tri.frag

windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -o bin/abyssal_drifter.exe
