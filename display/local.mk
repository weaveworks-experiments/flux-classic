display/main.css: display/main.less
	lessc display/main.less display/main.css

.PHONY: clean-display
clean: clean-display

clean-display:
	rm -f display/main.css
