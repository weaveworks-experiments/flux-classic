DISPLAY_BABELGEN:=$(patsubst %.babel,%.js,$(shell find display/gen -name '*.babel'))
DISPLAY_LESSGEN:=$(patsubst %.less,%.css,$(shell find display/gen -name '*.less'))
DISPLAY_GEN:=$(DISPLAY_LESSGEN) $(DISPLAY_BABELGEN)
DISPLAY_STATIC:=display/index.html display/res/*.css display/res/*.js

display.bin: $(shell find display -name '*.go')

docker/.display.done: $(DISPLAY_STATIC) $(DISPLAY_GEN)

$(DISPLAY_GEN): docker/.webbuild.done

%.css: %.less
	$(call run_build_container,webbuild,,,,lessc $< $@)

%.js: %.babel
	$(call run_build_container,webbuild,,,,babel $< -o $@)

.PHONY: clean-display
clean: clean-display

clean-display:
	rm -f $(DISPLAY_GEN)
