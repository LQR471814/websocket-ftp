"use strict";
exports.__esModule = true;
var upload_1 = require("../client/upload");
var fs_1 = require("fs");
var path_1 = require("path");
var yargs_1 = require("yargs");
var helpers_1 = require("yargs/helpers");
var argv = (0, yargs_1["default"])((0, helpers_1.hideBin)(process.argv)).options({
    url: { type: 'string', demandOption: true },
    files: { type: 'string', array: true, demandOption: true },
    timeout: { type: 'number', "default": -1 }
}).parseSync();
console.log("URL: ".concat(argv.url, " Files: ").concat(argv.files));
var t = new upload_1.Transfer(argv.url, argv.files.map(function (path) {
    var b = (0, fs_1.readFileSync)(path);
    return {
        Name: (0, path_1.basename)(path),
        Size: b.length,
        Type: "application/octet-stream",
        data: b
    };
}), {
    onstart: function () {
        if (argv.timeout > 0)
            setTimeout(t.cancel, argv.timeout);
    },
    onprogress: function (r, t) { return console.log("Sent ".concat(r, " / ").concat(t, " bytes")); },
    onsuccess: function () {
        console.log("Successfully completed transfer");
    },
    onclose: function () {
        console.log("Closed WS connection");
        process.exit();
    }
}, true);
