package task

import (
	"errors"
	"fmt"
	"strconv"
)

type parser struct {
	s   string
	pos int
}

func (p *parser) peek() byte {
	for p.pos < len(p.s) && p.s[p.pos] == ' ' {
		p.pos++
	}
	if p.pos < len(p.s) {
		return p.s[p.pos]
	}
	return 0
}

func (p *parser) expr() (float64, error) {
	v, err := p.term()
	for err == nil {
		op := p.peek()
		if op != '+' && op != '-' {
			break
		}
		p.pos++
		var r float64
		if r, err = p.term(); op == '+' {
			v += r
		} else {
			v -= r
		}
	}
	return v, err
}

func (p *parser) term() (float64, error) {
	v, err := p.factor()
	for err == nil {
		op := p.peek()
		if op != '*' && op != '/' {
			break
		}
		p.pos++
		var r float64
		if r, err = p.factor(); err != nil {
			break
		}
		if op == '*' {
			v *= r
		} else if r == 0 {
			return 0, errors.New("division by zero")
		} else {
			v /= r
		}
	}
	return v, err
}

func (p *parser) factor() (float64, error) {
	switch c := p.peek(); {
	case c == '-':
		p.pos++
		v, err := p.factor()
		return -v, err
	case c == '(':
		p.pos++
		v, err := p.expr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, errors.New("missing )")
		}
		p.pos++
		return v, nil
	case (c >= '0' && c <= '9') || c == '.':
		start := p.pos
		for p.pos < len(p.s) && ((p.s[p.pos] >= '0' && p.s[p.pos] <= '9') || p.s[p.pos] == '.') {
			p.pos++
		}
		return strconv.ParseFloat(p.s[start:p.pos], 64)
	}
	return 0, fmt.Errorf("unexpected input at %d", p.pos)
}

func Eval(expr string) (float64, error) {
	p := &parser{s: expr}
	v, err := p.expr()
	if err != nil {
		return 0, err
	}
	if p.peek() != 0 {
		return 0, fmt.Errorf("trailing input at %d", p.pos)
	}
	return v, nil
}
