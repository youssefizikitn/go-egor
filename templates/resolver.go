package templates

import (
	"errors"
	"fmt"
)

const CppTemplate = `
//
// Created by egor http://github.com/chermehdi/egor
// {{if .Author }}
// @author {{ .Author }}
{{end}}
#include <iostream>
#include <vector>
#include <set>
#include <map>
#include <algorithm>
#include <cmath>

using namespace std;

int main() {

}
`
const JavaTemplate = `
import java.util.*;
import java.io.*;

/**
 * Made by egor https://github.com/chermehdi/egor.
 * {{if .Author }}
 * @author {{ .Author }}
 * {{end}}
 */
public class Main {

    void solve(Scanner in, PrintWriter out) {

    }

    public static void main(String[] args) {
        try(Scanner in = new Scanner(System.in);
            PrintWriter out = new PrintWriter(System.out)) {
            new Main().solve(in, out);
        }
    }
}
`

const PythonTemplate = `
#
# Created by egor http://github.com/chermehdi/egor
# {{if .Author }}
# @author {{ .Author }}
# {{end}}
`

func ResolveTemplateByLanguage(lang string) (string, error) {
	switch lang {
	case "cpp":
		return CppTemplate, nil
	case "java":
		return JavaTemplate, nil
	case "python":
		return PythonTemplate, nil
	default:
		return "", errors.New(fmt.Sprintf("Unknown language %s provided", lang))
	}
}
