.PHONY: shaders

all:

shaders:
	glslc -o tri.vert.spv tri.vert
	glslc -o tri.frag.spv tri.frag
