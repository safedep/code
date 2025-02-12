const fs = require('fs');
const https = require('https');

https.get('https://api.example.com/data-fss', (resp) => {
    let data = '';

    resp.on('data', (chunk) => {
        data += chunk;
    });

    resp.on('end', () => {
        fs.writeFileSync('output.txt', data);
    });

}).on("error", (err) => {
    console.log("Error: " + err.message);
});