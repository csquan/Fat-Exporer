package main

import (
	"eth2-exporter/utils"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

var cssVendor = []string{"/css/layout.css", "/css/fontawesome.min.css", "/css/layout/toggle.css", "/css/layout/banner.css"}

var jsVendor = []string{"/bundle/js/jquery.min.js", "/bundle/js/popper.min.js", "/theme/js/bootstrap.min.js", "/bundle/js/luxon.min.js", "/bundle/js/typeahead.bundle.min.js", "/bundle/js/layout.js", "/bundle/js/banner.js", "/bundle/js/clipboard.min.js", "/bundle/js/instant.min.js", "/bundle/js/revive.min.js"}

func bundle(staticDir string) error {
	if staticDir == "" {
		staticDir = "./static"
	}

	fileInfo, err := os.Stat(staticDir)
	if err != nil {
		return fmt.Errorf("error getting stats about the static dir", err)
	}

	if !fileInfo.IsDir() {
		return fmt.Errorf("error static dir is not a directory")
	}

	bundleDir := path.Join(staticDir, "bundle")
	if _, err := os.Stat(bundleDir); os.IsNotExist(err) {
		os.Mkdir(bundleDir, 0755)
	} else if err != nil {
		return fmt.Errorf("error getting stats about the bundle dir", err)
	}

	type fileType struct {
		ext       string
		transform api.TransformOptions
	}

	types := []fileType{
		{
			ext: "css",
			transform: api.TransformOptions{
				Loader:            api.LoaderCSS,
				MinifyWhitespace:  true,
				MinifyIdentifiers: false,
				MinifySyntax:      true,
			},
		},
		{
			ext: "js",
			transform: api.TransformOptions{
				Loader:            api.LoaderJS,
				MinifyWhitespace:  true,
				MinifyIdentifiers: false,
				MinifySyntax:      true,
				Target:            api.ES2016,
			},
		},
	}

	for _, fileType := range types {
		bundleTypeDir := path.Join(bundleDir, fileType.ext)
		typeDir := path.Join(staticDir, fileType.ext)
		matches, err := utils.Glob(typeDir, "."+fileType.ext)

		if err != nil {
			return err
		}

		for _, match := range matches {
			code, err := ioutil.ReadFile(match)
			if err != nil {
				return fmt.Errorf("error reading file %v", err)
			}
			if !strings.Contains(match, ".min") {
				content := string(code)
				result := api.Transform(content, fileType.transform)
				if len(result.Errors) != 0 {
					return fmt.Errorf("error transforming %v %v", fileType, result.Errors)
				}
				code = result.Code
			}
			match = strings.Replace(match, typeDir, bundleTypeDir, -1)

			if _, err := os.Stat(path.Dir(match)); os.IsNotExist(err) {
				os.Mkdir(path.Dir(match), 0755)
			}

			err = ioutil.WriteFile(match, code, 0755)
			if err != nil {
				return fmt.Errorf("error failed to write file %v", err)
			}
		}
	}

	return nil
}

func vendorJS(staticDir string) error {

	jsCode := make([]byte, 0)
	for _, filePath := range jsVendor {
		code, err := ioutil.ReadFile(path.Join(staticDir, filePath))
		if err != nil {
			return fmt.Errorf("error reading file %v", err)
		}
		jsCode = append(jsCode, code...)
		jsCode = append(jsCode, byte('\n'))
	}

	result := api.Transform(string(jsCode), api.TransformOptions{
		Loader:            api.LoaderJS,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		Target:            api.ES2016,
	})

	if len(result.Errors) > 0 {
		return fmt.Errorf("error occured creating js vendor bundle %v", result.Errors)
	}

	err := ioutil.WriteFile(path.Join(staticDir, "bundle", "vendor.js"), result.Code, 0755)
	if err != nil {
		return fmt.Errorf("error failed to write file %v", err)
	}

	return nil
}

func vendorCSS(staticDir string) error {

	jsCode := make([]byte, 0)
	for _, filePath := range cssVendor {
		code, err := ioutil.ReadFile(path.Join(staticDir, filePath))
		if err != nil {
			return fmt.Errorf("error reading file %v", err)
		}
		jsCode = append(jsCode, code...)
		jsCode = append(jsCode, byte('\n'))
	}

	result := api.Transform(string(jsCode), api.TransformOptions{
		Loader:            api.LoaderCSS,
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
	})

	if len(result.Errors) > 0 {
		return fmt.Errorf("error occured creating js vendor bundle %v", result.Errors)
	}

	err := ioutil.WriteFile(path.Join(staticDir, "bundle", "vendor.css"), result.Code, 0755)
	if err != nil {
		return fmt.Errorf("error failed to write file %v", err)
	}

	return nil
}

func main() {
	if err := bundle("./static"); err != nil {
		log.Fatal("error bundling: ", err)
	}

	if err := vendorJS("./static"); err != nil {
		log.Fatal("error creating vendor files", err)
	}

	if err := vendorCSS("./static"); err != nil {
		log.Fatal("error creating vendor files", err)
	}
}
