## Overview

mustache.go is an implementation of the mustache template language in Go. It is better suited for website templates than Go's native pkg/template. mustache.go is fast -- it parses templates efficiently and stores them in a tree-like structure which allows for fast execution. 

this fork make two new points:

* Nested visit:  {{A.B.C}}, {{#A.B}}{{C}}{{/A.B}} can be use.

* Template Inheritance: a template can inherit a parent template, see below.


## Documentation

For more information about mustache, check out the [mustache project page](http://github.com/defunkt/mustache) or the [mustache manual](http://mustache.github.com/mustache.5.html).

Also check out some [example mustache files](http://github.com/defunkt/mustache/tree/master/examples/)

## Installation
To install mustache.go, simply run `go get github.com/jander/mustache`. To use it in a program, use `import "github.com/jander/mustache"`

## Usage
There are four main methods in this package:

    func Render(data string, context ...interface{}) string
    
    func RenderFile(filename string, context ...interface{}) string
    
    func ParseString(data string) (*template, os.Error)
    
    func ParseFile(filename string) (*template, os.Error) 

There are also two additional methods for using layouts (explained below).

The Render method takes a string and a data source, which is generally a map or struct, and returns the output string. If the template file contains an error, the return value is a description of the error. There's a similar method, RenderFile, which takes a filename as an argument and uses that for the template contents. 

    data := mustache.Render("hello {{c}}", map[string]string{"c":"world"})
    println(data)


If you're planning to render the same template multiple times, you do it efficiently by compiling the template first:

    tmpl,_ := mustache.ParseString("hello {{c}}")
    var buf bytes.Buffer;
    for i := 0; i < 10; i++ {
        tmpl.Render (map[string]string { "c":"world"}, &buf)  
    }

For more example usage, please see `mustache_test.go`

## Escaping

mustache.go follows the official mustache HTML escaping rules. That is, if you enclose a variable with two curly brackets, `{{var}}`, the contents are HTML-escaped. For instance, strings like `5 > 2` are converted to `5 &gt; 2`. To use raw characters, use three curly brackets `{{{var}}}`.


## Template Inheritance

Get inspiration from jinja2(http://jinja.pocoo.org/), a template can inherit a parent template. Template inheritance allows you to build a base “skeleton” template that contains all the common elements of your site and defines blocks that child templates can override.

use {{<ParentTemplate}} to declare a template inherit a parent template.

use {{*BlockName}} to declare a block.

layout.html:

    <html>
    <head><title>{{*title}}Hi{{/title}}</title></head>
    <body>
    {{*body}}
    this content will be replace with child definition.
    {{/body}}
    </body>
    </html>

child.html

    {{<layout}}
    {{*title}}child title{{/title}}
    {{*body}}<h1> Hello World! {{/body}}

A call to `RenderFile("child.html", nil)` will produce:

    <html>
    <head><title>child title</title></head>
    <body>
    <h1> Hello World! </h1>
    </body>
    </html>


## A note about method receivers

Mustache.go supports calling methods on objects, but you have to be aware of Go's limitations. For example, lets's say you have the following type:

    type Person struct {
        FirstName string
        LastName string    
    }

    func (p *Person) Name1() string {
        return p.FirstName + " " + p.LastName
    }

    func (p Person) Name2() string {
        return p.FirstName + " " + p.LastName
    }

While they appear to be identical methods, `Name1` has a pointer receiver, and `Name2` has a value receiver. Objects of type `Person`(non-pointer) can only access `Name2`, while objects of type `*Person`(person) can access both. This is by design in the Go language.

So if you write the following:

    mustache.Render("{{Name1}}", Person{"John", "Smith"})

It'll be blank. You either have to use `&Person{"John", "Smith"}`, or call `Name2`

## Supported features

* Variables
* Comments
* Sections (boolean, enumerable, and inverted)
* Partials

## New Points

* Nested visit
* Template Inheritance

