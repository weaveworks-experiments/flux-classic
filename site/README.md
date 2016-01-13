# Documentation site

## To run locally

This documentation site uses
[Jekyll](https://github.com/jekyll/jekyll) to generate the web pages,
and bundler [Bundler](http://bundler.io) (see there for how to install
it) to manage dependencies, which both run on Ruby.

Once you have Ruby and bundler, do

```bash
bundler install
bundler exec jekyll serve
```

Now you should be able to visit http://localhost:4000/ to see the
site.

## Adding pages

Each file needs a header with the layout (usually `page`) and a title,
which will become a header. For example

```yaml
---
layout: page
title: fluxctl service
---
```

I have kept to a single level of directories for topics. Each
directory gets an index (addressable at the URL `/topic/`) and may
have other files.

Please add new pages to the navigation, in `_includes/content.html`.
