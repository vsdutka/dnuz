// Copyright © 2018 Vyacheslav Dutka
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile        string
	srcUrl         string
	nonUtf8EncName string
	outEncName     string
	outPath        string
	decoder        transform.Transformer
	encoder        transform.Transformer
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "dnuz",
	Short: "Download&unzip&fix non utf8 paths/filenames",
	Long:  `Download&unzip&fix non utf8 paths/filenames`,
	Args: func(cmd *cobra.Command, args []string) error {
		if srcUrl == "" {
			return errors.New("requires at least url")
		}
		var err error
		decoder, err = getDecoder(nonUtf8EncName)
		if err != nil {
			return err
		}
		encoder, err = getEncoder(outEncName)
		if err != nil {
			return err
		}
		return nil
	},
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		res, err := http.Get(srcUrl)
		if err != nil {
			log.Fatal(err)
		}
		defer res.Body.Close()
		d, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("ReadFile: Size of download: %d\n", len(d))
		files, err := func() ([]string, error) {
			var filenames []string

			r, err := zip.NewReader(bytes.NewReader(d), int64(len(d)))
			//r, err := zip.OpenReader(src)
			if err != nil {
				return filenames, err
			}

			for _, f := range r.File {

				rc, err := f.Open()
				if err != nil {
					return filenames, err
				}
				defer rc.Close()

				// Store filename/path for returning and using later on
				fname := f.Name
				if f.NonUTF8 {
					if decoder != nil {
						// Разные кодировки = разные длины символов.
						newFName := make([]byte, len(fname)*2)
						n, _, err := decoder.Transform(newFName, []byte(fname), false)
						if err != nil {
							panic(err)
						}
						fname = string(newFName[:n])
					}
				}
				fpath := strings.ToLower(filepath.Join(outPath, fname))
				if encoder != nil {
					newFPath := make([]byte, len(fpath)*2)
					n, _, err := encoder.Transform(newFPath, []byte(fpath), false)
					if err != nil {
						panic(err)
					}
					fpath = string(newFPath[:n])
				}
				filenames = append(filenames, fpath)

				if f.FileInfo().IsDir() {

					// Make Folder
					os.MkdirAll(fpath, os.ModePerm)

				} else {

					// Make File
					if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
						return filenames, err
					}

					outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
					if err != nil {
						return filenames, err
					}

					_, err = io.Copy(outFile, rc)

					// Close the file without defer to close before next iteration of loop
					outFile.Close()

					if err != nil {
						return filenames, err
					}

				}
			}
			return filenames, nil
		}()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("Unzipped:\n" + strings.Join(files, "\n"))
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.dnuz.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	RootCmd.PersistentFlags().StringVar(&srcUrl, "src-url", "", "Source file url")
	RootCmd.PersistentFlags().StringVar(&outPath, "out-path", "", "Output path")
	RootCmd.PersistentFlags().StringVar(&nonUtf8EncName, "nonUtf8-enc", "", "Encoding name for nonUTF8 filenames")
	RootCmd.PersistentFlags().StringVar(&outEncName, "out-enc", "", "Encoding name for filenames")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".dnuz" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".dnuz")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func getEncoding(encName string) (encoding.Encoding, error) {
	if encName != "" {
		switch strings.ToLower(encName) {
		case "866":
			return charmap.CodePage866, nil
		case "cp866":
			return charmap.CodePage866, nil
		case "1251":
			return charmap.Windows1251, nil
		case "windows-1251":
			return charmap.Windows1251, nil
		default:
			return nil, errors.New("Unsupported encoding name\"" + encName + "\"")
		}
	}
	return encoding.Nop, nil
}

func getDecoder(encName string) (transform.Transformer, error) {
	e, err := getEncoding(encName)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, nil
	}
	return e.NewDecoder(), nil
}
func getEncoder(encName string) (transform.Transformer, error) {
	e, err := getEncoding(encName)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, nil
	}
	return e.NewEncoder(), nil
}
