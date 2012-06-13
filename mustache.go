package mustache

import (
	"io"
	"io/ioutil"
	"fmt"
	"html"
	"reflect"
	"strings"
	"bytes"
	"os"
	"path"
)

/*
==================================
mustache element
==================================

variable element:
	{{variable}}

raw, now escape html tag:
	{{{variable}}}

if or for:
	{{#variable}}{{/variable}}

not:
	{{^variable}}{{/variable}}

comment element:
	{{! comment words }}

include element:
	{{>include_name}}

function element:
	{{function}}
	{{#function}}{{/function}}

==================================
New pionts below
==================================
template inheritance:
	{{<template_name}}

block element:
	{{*block_name}}
*/

type element interface {
	Type() ElementType
	Render(writer io.Writer, contextChain []interface{})
	String() string
}

type container interface{
	element
	addChild(child element)
	getName() string
}

type ElementType int

func (t ElementType) Type() ElementType{
	return t
}

const (
	textType       ElementType = iota // Plain text.
	variableType
	sectionType
	templateType
	blockType
)


type textElement struct {
	ElementType
	text []byte
}

func (el *textElement) Render(writer io.Writer, contextChain []interface{}) {
	writer.Write(el.text)
}

func (el *textElement) String() string{
	return fmt.Sprintf("Text{ %q }", el.text)
}

type variableElement struct {
	ElementType
	name string
	raw bool
}

func (el *variableElement) Render(writer io.Writer, contextChain []interface{}) {
	val := lookup(el.name, contextChain)
	if val.IsValid(){
		fmt.Println("raw", el.raw)
		if el.raw{
			fmt.Fprint(writer, val.Interface())
		}else{
			s := fmt.Sprint(val.Interface())
			fmt.Fprint(writer, html.EscapeString(s))
		}
	}
}

func (el *variableElement) String() string{
	return fmt.Sprintf("Variable{ name=%s, raw=%v }", el.name, el.raw)
}

type sectionElement struct {
	ElementType
	name string
	inverted bool
	elements []element
}

func (el *sectionElement) String() string{
	return fmt.Sprintf("Section{ name=%s, inverted=%v, elements=%v }", el.name, el.inverted, el.elements)
}

func (el *sectionElement) addChild(child element) {
	el.elements = append(el.elements, child)
}

func (el *sectionElement) getName() string{
	return el.name
}

func (section *sectionElement) Render(writer io.Writer, contextChain []interface{}) {
	val := lookup(section.name, contextChain)

	var ctxs = []interface{}{}
	// if the value is nil, check if it's an inverted section
	isTrue := isTrue(val)
	if !isTrue && !section.inverted || isTrue && section.inverted {
		return
	} else {
		switch val.Kind() {
		case reflect.Slice:
			for i := 0; i < val.Len(); i++ {
				ctxs = append(ctxs, val.Index(i).Interface())
			}
		case reflect.Array:
			for i := 0; i < val.Len(); i++ {
				ctxs = append(ctxs, val.Index(i).Interface())
			}
		//case reflect.Map, reflect.Struct:
		default:
			if val.IsValid(){
				ctxs = append(ctxs, val.Interface())
			}else{
				ctxs = append(ctxs, "")
			}
		}
	}

	chain := make([]interface{}, len(contextChain)+1)
	copy(chain[1:], contextChain)

	for _, ctx := range ctxs {
		chain[0] = ctx
		for _, el := range section.elements {
			el.Render(writer, chain)
		}
	}
}


type blockElement struct{
	ElementType
	name string
	elements []element
}

func (el *blockElement) String() string{
	return fmt.Sprintf("block{ name=%s, elements=%v }", el.name, el.elements)
}

func (el *blockElement) addChild(child element) {
	el.elements = append(el.elements, child)
}

func (el *blockElement) getName() string{
	return el.name
}

func (el *blockElement) Render(writer io.Writer, contextChain []interface{}) {
	for _, el := range el.elements {
		el.Render(writer, contextChain)
	}
}


type Template struct {
	ElementType
	data []byte
	leftToken string
	rightToken string
	pos int
	line int
	dir string                         // template file dir
	elements []element                 // children elements
	blocks map[string]*blockElement    // blcok map
	ext string                         // file extension
	parent string                      // parent template name
}

func (el *Template) String() string{
	return fmt.Sprintf("\nTemplate{elements=%v,\n blocks=%v,\n parent=%v}\n", el.elements, el.blocks, el.parent)
}

func (el *Template) addChild(child element){
	el.elements = append(el.elements, child)
}

func (el *Template) getName() string{
	return "template"
}

func (tmpl *Template) Render(writer io.Writer, contextChain []interface{}) {
	atmpl := tmpl
	if tmpl.parent != "" {
		tmpls := []*Template{tmpl}
		
		for{
			if atmpl.parent != "" {
				filename := path.Join(tmpl.dir, atmpl.parent + tmpl.ext)
				parentTmpl, err := ParseFile(filename)
				if err != nil{
					panic(err)
				}
				tmpls = append(tmpls, parentTmpl)
				atmpl = parentTmpl
				
			}else{
				break
			}
		}
		atmpl = tmpl
		for i:= 1; i< len(tmpls); i++{
			for _, block := range atmpl.blocks{
				_, found := tmpls[i].blocks[block.getName()]
				if found{
					tmpls[i].blocks[block.getName()] = block
				}
			}
			
			atmpl = tmpls[i]
		}
		atmpl = tmpls[len(tmpls)-1]
	}
	
	for _, el := range atmpl.elements {
		if el.Type() != blockType{
			el.Render(writer, contextChain)
		}else{
			atmpl.blocks[el.(*blockElement).getName()].Render(writer, contextChain)
		}
	}
}

func (tmpl *Template) RenderToString(contexts ...interface{}) string{
	var buf bytes.Buffer
	tmpl.Render(&buf, contexts)
	return buf.String()
}

// goto the next token, and return the passed bytes.
func (tmpl *Template) nextToken(token string) (text []byte, err error){
	i := tmpl.pos

	for{
		if i + len(token) > len(tmpl.data) {
			return tmpl.data[tmpl.pos:], io.EOF
		}
		
		b := tmpl.data[i]

		if b == '\n' {
			tmpl.line++
		}

		if b != token[0] {
			i++
			continue
		}
		
		if bytes.HasPrefix(tmpl.data[i+1:], []byte(token[1:])){
			//match
			text := tmpl.data[tmpl.pos:i]
			tmpl.pos = i + len(token)
			return text, nil
		}
		i++
	}
	return []byte{}, nil
}

func (tmpl *Template) parse() error{
	for{
		text, err := tmpl.nextToken(tmpl.leftToken)

		if len(text) >0{
			// add a text element
			tmpl.addChild(&textElement{textType, text})
		}
		
		if err == io.EOF {
			return nil
		}

		// prepare the next token
		token := tmpl.rightToken
		if tmpl.pos < len(tmpl.data) && tmpl.data[tmpl.pos] == '{' {
			// is raw variable element
			token = tmpl.rightToken + "}"
		}

		text, err = tmpl.nextToken(token)

		if err == io.EOF{
			return parseError{tmpl.line, "unmatched open tag"}
		}

		text = bytes.TrimSpace(text)
		if len(text) == 0{
			return parseError{tmpl.line, "empty tag"}
		}

		// check the kind of element
		switch text[0]{
		case '!':
			// ignore comment
			break
		case '#', '^':
			// section element
			if tmpl.parent != ""{
				break
			}
			name := string(bytes.TrimSpace(text[1:]))

			//ignore the new line when section start
			if tmpl.pos < len(tmpl.data) && tmpl.data[tmpl.pos] == '\n'{
				tmpl.pos += 1
			}else if tmpl.pos+1 < len(tmpl.data) && tmpl.data[tmpl.pos] == '\r' && tmpl.data[tmpl.pos+1] == '\n'{
				tmpl.pos += 2
			}

			section := &sectionElement{sectionType, name, text[0]=='^', []element{}}

			if err = tmpl.pareseContainer(section); err!=nil{
				return err
			}

			tmpl.addChild(section)

		case '{':
			if tmpl.parent != ""{
				break
			}
			// raw tag
			tmpl.addChild(&variableElement{variableType, string(text[1:]), true})

		case '>':
			// partial
			if tmpl.parent != ""{
				break
			}
			name := string(bytes.TrimSpace(text[1:]))
			partial, err := tmpl.parsePartial(name)
			if err != nil {
				return err
			}
			tmpl.addChild(partial)

		case '<':
			// parent template
			name := string(bytes.TrimSpace(text[1:]))
			tmpl.parent = name

		case '*':
			// block element
			name := string(bytes.TrimSpace(text[1:]))

			block := &blockElement{blockType, name, []element{}}

			if err = tmpl.pareseContainer(block); err!=nil{
				return err
			}

			tmpl.addChild(block)
			tmpl.blocks[name] = block

		case '/':
			return parseError{tmpl.line, "unmatched close tag"}
		default:
			if tmpl.parent != ""{
				break
			}
			tmpl.addChild(&variableElement{variableType, string(text), false})
		}
	}
	return nil
}


func (tmpl *Template) pareseContainer(el container) error{
	for{
		text, err := tmpl.nextToken(tmpl.leftToken)

		if err == io.EOF{
			return parseError{tmpl.line, el.getName() + " has no closing tag"}
		}

		// add a text element
		if len(text)>0{
			el.addChild(&textElement{textType, text})
		}

		// next token
		token := tmpl.rightToken
		if tmpl.pos < len(tmpl.data) && tmpl.data[tmpl.pos] == '{' {
			// is raw variable element
			token = tmpl.rightToken + "}"
		}

		text, err = tmpl.nextToken(token)
		if err == io.EOF{
			return parseError{tmpl.line, "unmatched open tag"}
		}

		text = bytes.TrimSpace(text)
		if len(text) == 0{
			return parseError{tmpl.line, "empty tag"}
		}

		switch text[0]{
		case '!', '<':
			// ignore
			break

		case '#', '^':
			// section element
			name := string(bytes.TrimSpace(text[1:]))

			//ignore the new line when section start
			if tmpl.pos < len(tmpl.data) && tmpl.data[tmpl.pos] == '\n' {
				tmpl.pos += 1
			} else if tmpl.pos+1 < len(tmpl.data) && tmpl.data[tmpl.pos] == '\r' && tmpl.data[tmpl.pos+1] == '\n' {
				tmpl.pos += 2
			}

			sec := &sectionElement{sectionType, name, text[0]=='^', []element{}}

			if err = tmpl.pareseContainer(sec); err!=nil{
				return err
			}
			
			el.addChild(sec)

		case '{':
			// raw tag
			el.addChild(&variableElement{variableType, string(text[1:]), true})

		case '>':
			// partial element
			name := string(bytes.TrimSpace(text[1:]))
			partial, err := tmpl.parsePartial(name)
			if err != nil {
				return err
			}
			el.addChild(partial)

		case '*':
			// block element
			name := string(bytes.TrimSpace(text[1:]))
			block := &blockElement{blockType, name, []element{}}

			if err = tmpl.pareseContainer(block); err!=nil{
				return err
			}

			el.addChild(block)
			tmpl.blocks[name] = block

		case '/':
			// close element
			name := string(bytes.TrimSpace(text[1:]))
			if name != el.getName() {
				return parseError{tmpl.line, "error closing tag: " + name}
			} else {
				return nil
			}
		default:
			el.addChild(&variableElement{variableType, string(text), false})
		}
	}
	return nil
}


func (tmpl *Template) parsePartial(name string) (*Template, error) {
	filename := path.Join(tmpl.dir, name + tmpl.ext)
	partial, err := ParseFile(filename)

	if err != nil {
		return nil, err
	}
	return partial, nil
}


func ParseString(data string) (*Template, error) {
	cwd := os.Getenv("CWD")
	tmpl := Template{templateType, []byte(data), "{{", "}}", 0, 1, cwd, []element{}, map[string]*blockElement{}, "", ""}
	err := tmpl.parse()

	if err != nil {
		return nil, err
	}

	return &tmpl, err
}


func ParseFile(filename string) (*Template, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	dirname, fname := path.Split(filename)
	ext := path.Ext(fname)

	tmpl := Template{templateType, data, "{{", "}}", 0, 1, dirname, []element{}, map[string]*blockElement{}, ext, ""}

	err = tmpl.parse()
	if err != nil {
		return nil, err
	}

	return &tmpl, nil
}

func Render(data string, context ...interface{}) string {
	tmpl, err := ParseString(data)
	if err != nil {
		return err.Error()
	}
	return tmpl.RenderToString(context...)
}

func RenderFile(filename string, context ...interface{}) string {
	tmpl, err := ParseFile(filename)
	if err != nil {
		return err.Error()
	}
	return tmpl.RenderToString(context...)
}


//=============================================
// helper fuction
//=============================================


type parseError struct {
	line    int
	message string
}

func (p parseError) Error() string {
	return fmt.Sprintf("line %d: %s", p.line, p.message)
}


func lookupAttr(name string, contextChain []interface{}) (reflect.Value) {
	Outer:
	for _, ctx := range contextChain{

		v := reflect.ValueOf(ctx)

		for v.IsValid(){
			// check func
			t := v.Type()
			if n := v.Type().NumMethod(); n > 0 {
				for i := 0; i < n; i++ {
					m := t.Method(i)
					if m.Name == name && m.Type.NumIn() == 1 {
						return v.Method(i).Call(nil)[0]
					}
				}
			}

			switch av:=v; av.Kind(){

			case reflect.Ptr, reflect.Interface:
				v = av.Elem()

			case reflect.Struct:
				ret := av.FieldByName(name)
				if ret.IsValid() {
					if ret.Kind() == reflect.Interface || ret.Kind() == reflect.Ptr{
						ret = ret.Elem()
					}
					return ret
				} else {
					continue Outer
				}

			case reflect.Map:
				ret := av.MapIndex(reflect.ValueOf(name))

				if ret.IsValid() {
					if ret.Kind() == reflect.Interface || ret.Kind() == reflect.Ptr{
						ret = ret.Elem()
					}
					return ret
				} else {
					continue Outer
				}

			default:
				continue Outer
			}
		}
	}
	return reflect.Value{}
}

func lookup(name string, contextChain []interface{}) reflect.Value {
	var v = reflect.Value{}

	var ctxs = make([]interface{}, len(contextChain)+1)
	copy(ctxs[1:], contextChain)


	for idx, attr := range strings.Split(name, "."){

		if idx>0{
			ctxs[0] = v.Interface()
			v = lookupAttr(attr, ctxs)
		}else{
			v = lookupAttr(attr, contextChain)
		}

		if !v.IsValid(){
			break
		}
	}
	return v
}


func isTrue(val reflect.Value) bool {
	if !val.IsValid() {
		return false
	}
	switch val.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return val.Len() > 0
	case reflect.Bool:
		return val.Bool()
	case reflect.Complex64, reflect.Complex128:
		return val.Complex() != 0
	case reflect.Chan, reflect.Func, reflect.Ptr, reflect.Interface:
		return !val.IsNil()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int() != 0
	case reflect.Float32, reflect.Float64:
		return val.Float() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return val.Uint() != 0
	case reflect.Struct:
		return true // Struct values are always true.
	}
	return false
}