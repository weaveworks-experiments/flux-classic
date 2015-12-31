$(call image_stamp,edgebal): edgebal/supervisord.conf edgebal/nginx.tmpl edgebal/nginx.conf
$(call image_stamp,edgebal): edgebal/unavailable.html
$(call image_stamp,edgebal): build/bin/balagent
