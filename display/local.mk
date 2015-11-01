DISPLAY_BABELGEN:=$(patsubst %.babel,%.js,$(shell find display/gen -name '*.babel'))
DISPLAY_LESSGEN:=$(patsubst %.less,%.css,$(shell find display/gen -name '*.less'))
DISPLAY_STATIC:=display/index.html display/res/*.css display/res/*.js

docker/.display.done: $(DISPLAY_STATIC) $(DISPLAY_LESSGEN) $(DISPLAY_BABELGEN)

%.css: %.less
	$(call run_build_container,,,,lessc $^ $@)

%.js: %.babel
	$(call run_build_container,,,,babel $^ -o $@)

.PHONY: clean-display
clean: clean-display

clean-display:
	rm -f $(DISPLAY_BABELGEN) $(DISPLAY_LESSGEN)
