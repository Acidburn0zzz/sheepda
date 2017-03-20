// Copyright (C) 2017 JT Olds
// See LICENSE for copying information.

package sheepda

import (
	"bytes"
	"fmt"
	"io"
	"unicode"
)

var (
	Lambdas = map[rune]bool{
		'Λ': true, 'λ': true, 'ᴧ': true, 'Ⲗ': true, 'ⲗ': true, '𝚲': true,
		'𝛌': true, '𝛬': true, '𝜆': true, '𝜦': true, '𝝀': true, '𝝠': true,
		'𝝺': true, '𝞚': true, '𝞴': true, '\\': true,
	}
)

func IsVariableRune(ch rune) bool {
	return !unicode.IsSpace(ch) && ch != '(' && ch != ')' && ch != '.' &&
		ch != '=' && !Lambdas[ch]
}

func ParseVariable(s *Stream) (name string, err error) {
	for {
		ch, err := s.Peek()
		if err != nil {
			if err == io.EOF && name != "" {
				break
			}
			return "", err
		}
		if !IsVariableRune(ch) {
			break
		}
		name += string(ch)
		s.Next()
	}
	if name == "" {
		return "", fmt.Errorf("variable expected, not found")
	}
	return name, s.SwallowWhitespace()
}

type Expr interface {
	String() string
}

type LambdaExpr struct {
	Arg  string
	Body Expr
}

func (e *LambdaExpr) String() string {
	return fmt.Sprintf("λ%s.%s", e.Arg, e.Body)
}

func ParseLambda(s *Stream) (*LambdaExpr, error) {
	err := s.AssertMatch(Lambdas)
	if err != nil {
		return nil, err
	}
	arg, err := ParseVariable(s)
	if err != nil {
		return nil, err
	}
	err = s.AssertMatch(map[rune]bool{'.': true})
	if err != nil {
		return nil, err
	}
	body, err := ParseExpr(s)
	if err != nil {
		return nil, err
	}
	return &LambdaExpr{Arg: arg, Body: body}, nil
}

type ApplicationExpr struct {
	Func Expr
	Arg  Expr
}

func (e *ApplicationExpr) String() string {
	return fmt.Sprintf("(%s %s)", e.Func, e.Arg)
}

func ParseApplication(s *Stream) (*ApplicationExpr, error) {
	err := s.AssertMatch(map[rune]bool{'(': true})
	if err != nil {
		return nil, err
	}
	fn, err := ParseExpr(s)
	if err != nil {
		return nil, err
	}
	arg, err := ParseExpr(s)
	if err != nil {
		return nil, err
	}
	result := &ApplicationExpr{Func: fn, Arg: arg}
	for {
		r, err := s.Peek()
		if err != nil {
			return nil, err
		}
		if r == ')' {
			s.Next()
			return result, s.SwallowWhitespace()
		}
		next, err := ParseExpr(s)
		if err != nil {
			return nil, err
		}
		result = &ApplicationExpr{Func: result, Arg: next}
	}
}

type VariableExpr struct {
	Name string
}

func (e *VariableExpr) String() string {
	return e.Name
}

func ParseExpr(s *Stream) (Expr, error) {
	r, err := s.Peek()
	if err != nil {
		return nil, err
	}

	if Lambdas[r] {
		return ParseLambda(s)
	}
	if r == '(' {
		return ParseApplication(s)
	}
	if IsVariableRune(r) {
		name, err := ParseVariable(s)
		return &VariableExpr{Name: name}, err
	}

	return nil, fmt.Errorf("expression not found")
}

type assignment struct {
	LHS string
	RHS Expr
}

type ProgramExpr struct {
	Expr
}

func (e *ProgramExpr) String() string {
	var out bytes.Buffer
	expr := e.Expr
	applications := false
	for {
		if t, ok := expr.(*ApplicationExpr); ok {
			if fn, ok := t.Func.(*LambdaExpr); ok {
				fmt.Fprintf(&out, "%s = %s\n", fn.Arg, t.Arg)
				expr = fn.Body
				applications = true
				continue
			}
		}
		if applications {
			fmt.Fprintln(&out)
		}
		fmt.Fprint(&out, expr)
		return out.String()
	}
}

func Parse(s *Stream) (*ProgramExpr, error) {
	err := s.SwallowWhitespace()
	if err != nil {
		return nil, err
	}
	var assignments []assignment
	for {
		expr, err := ParseExpr(s)
		if err != nil {
			return nil, err
		}
		if s.EOF() {
			for i := len(assignments) - 1; i >= 0; i-- {
				expr = &ApplicationExpr{
					Func: &LambdaExpr{Arg: assignments[i].LHS, Body: expr},
					Arg:  assignments[i].RHS,
				}
			}
			return &ProgramExpr{Expr: expr}, nil
		}
		t, ok := expr.(*VariableExpr)
		if !ok {
			return nil, fmt.Errorf("unparsed code remaining")
		}
		err = s.AssertMatch(map[rune]bool{'=': true})
		if err != nil {
			return nil, err
		}
		rhs, err := ParseExpr(s)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment{LHS: t.Name, RHS: rhs})
	}
}
