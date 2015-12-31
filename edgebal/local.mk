$(call image_stamp,edgebal): edgebal/supervisord.conf build/bin/balagent

build/bin/balagent: $(balancer_go_srcs)
