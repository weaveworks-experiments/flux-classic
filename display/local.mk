DISPLAY_STATIC:=display/index.html display/res/*.css display/res/*.js display/main.css

docker/.display.done: $(DISPLAY_STATIC)

display/main.css: display/main.less
	lessc display/main.less display/main.css

.PHONY: clean-display
clean: clean-display

clean-display:
	rm -f display/main.css
