var express = require('express');
var httpProxy = require('http-proxy');
var url = require('url');

var app = express();


/************************************************************
 *
 * Express routes for:
 *   - app.js
 *   - index.html
 *
 *   Proxy requests to:
 *     - /api -> :7070/api
 *   Or mock api
 *     - /api/services
 *
 ************************************************************/


// Serve application file depending on environment
app.get(/app.js/, function(req, res) {
  var filename = req.originalUrl.split('/').pop();
  if (process.env.NODE_ENV === 'production') {
    res.sendFile(__dirname + '/build/' + filename);
  } else {
    res.redirect('//localhost:7071/build/' + filename);
  }
});

// Proxy to backend

var BACKEND_HOST = process.env.BACKEND_HOST;
var proxy;

if (process.env.BACKEND_HOST) {
  proxy = httpProxy.createProxy({
    target: 'http://' + BACKEND_HOST
  });

  proxy.on('error', function(err) {
    console.error('Proxy error', err);
  });

  app.all('/api*', proxy.web.bind(proxy));
} else {
  //
  // MOCK BACKEND
  //
  app.get('/api/services', function(req, res) {
    res.json(require('./support/services'));
  });
}

// Serve index page

app.use(express.static('src'));


/*************************************************************
 *
 * Webpack Dev Server
 *
 * See: http://webpack.github.io/docs/webpack-dev-server.html
 *
 *************************************************************/

if (process.env.NODE_ENV !== 'production') {
  var webpack = require('webpack');
  var WebpackDevServer = require('webpack-dev-server');
  var config = require('./webpack.local.config');

  new WebpackDevServer(webpack(config), {
    publicPath: config.output.publicPath,
    // contentBase: __dirname + '/src',
    hot: true,
    noInfo: true,
    historyApiFallback: true,
    stats: { colors: true }
  }).listen(7071, 'localhost', function (err, result) {
    if (err) {
      console.log(err);
    }
  });
}


/******************
 *
 * Express server
 *
 *****************/

var port = process.env.PORT || 7072;
var server = app.listen(port, function () {
  var host = server.address().address;
  var port = server.address().port;

  console.log('Flux UI listening at http://%s:%s', host, port);
});
