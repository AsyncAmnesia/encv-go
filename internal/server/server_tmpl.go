package server

var tmpl_dynamic_files = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>ENCV File Server - {{.CurrentPath}}</title>
    <style>
        body { font-family: sans-serif; background-color: #f4f4f9; color: #333; margin: 2em; }
        h1 { color: #444; }
        .breadcrumb { margin-bottom: 1em; }
        .breadcrumb a { color: #007BFF; text-decoration: none; }
        .breadcrumb a:hover { text-decoration: underline; }
        table { width: 100%; border-collapse: collapse; margin-top: 1em; }
        th, td { padding: 12px; border: 1px solid #ddd; text-align: left; }
        th { background-color: #4CAF50; color: white; }
        tr:nth-child(even) { background-color: #f2f2f2; }
        a { text-decoration: none; color: #007BFF; }
        a:hover { text-decoration: underline; }
        .dir-tag { color: #007BFF; font-weight: bold; }
        .container-tag { color: #d9534f; font-weight: bold; }
    </style>
</head>
<body>
    <h1>ENCV File Server</h1>
    <div class="breadcrumb">
        <a href="/">Root</a> / {{range .Ancestors}}<a href="{{.Path}}">{{.Name}}</a> / {{end}}
    </div>
    <table>
        <thead>
            <tr>
                <th>Name</th>
                <th>Type</th>
                <th>Size</th>
                <th>Modified</th>
            </tr>
        </thead>
        <tbody>
            {{if .NotRoot}}<tr><td><a href="{{.ParentPath}}">..</a></td><td>Directory</td><td>-</td><td>-</td></tr>{{end}}
            {{range .Files}}
            <tr>
                <td><a href="{{.Path}}">{{.Name}}</a></td>
                <td>
                    {{if .IsDir}}<span class="dir-tag">Directory</span>
                    {{else if .IsContainer}}<span class="container-tag">ENCV Container</span>
                    {{else}}File
                    {{end}}
                </td>
                <td>{{if .IsDir}}-{{else}}{{.Size}}{{end}}</td>
                <td>{{.ModTime.Format "2006-01-02 15:04:05"}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
</body>
</html>`
