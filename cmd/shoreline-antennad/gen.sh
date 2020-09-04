#!/bin/bash

TEMPLATE=

cat >indextmpl.go <<EOF 
package main

import (
    "html/template"
)

var tmpl = template.Must(template.New("index").Parse(\`$(cat index.html)\`))
EOF

go fmt .
