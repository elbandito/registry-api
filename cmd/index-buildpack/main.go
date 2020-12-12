package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

const MetadataLabel = "io.buildpacks.buildpackage.metadata"

//go:generate mockery --all --output=internal/mocks --case=underscore

type ImageFunction func(name.Reference, ...remote.Option) (v1.Image, error)

type Entry struct {
	Namespace string `json:"ns"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Address   string `json:"addr"`
}

type Metadata struct {
	ID       string
	Version  string
	Homepage string
	Stacks   []stack
}

type stack struct {
	ID string
}

type IndexRecord struct {
	entry    Entry
	metadata Metadata
	err      error
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("at=index_buildpack level=error msg='invalid inputs: expected entry json'")
		os.Exit(1)
	}

	data, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("at=index_buildpack level=error msg='invalid inputs: unable to read file' file='%s' reason='%s'\n", os.Args[1], err)
		os.Exit(1)
	}

	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		fmt.Printf("at=index_buildpack level=error msg='invalid inputs: unable to parse entry json' reason='%s'\n", err)
		os.Exit(1)
	}

	buildIndex(entries)
	fmt.Println("at=index_buildpack level=info msg='done updating index'")
}

func buildIndex(entries []Entry) {
	ch := make(chan IndexRecord)

	for _, e := range entries {
		go handleMetadata(e, remote.Image, ch)
	}

	for range entries {
		i := <-ch
		if i.err != nil {
			fmt.Printf("at=handleMetadata level=warn msg='failed to fetch config' entry='%s/%s@%s' reason='%s'\n", i.entry.Namespace, i.entry.Name, i.entry.Version, i.err)
		} else {
			err := UpdateOrInsertConfig(i.entry, i.metadata)
			if err != nil {
				fmt.Printf("at=buildIndex level=warn msg='failed to update index' entry='%s/%s@%s' reason='%s'\n", i.entry.Namespace, i.entry.Name, i.entry.Version, err)
			} else {
				fmt.Printf("at=buildIndex level=info msg='updated index' entry='%s/%s@%s'\n", i.entry.Namespace, i.entry.Name, i.entry.Version)
			}
		}
	}
}

func handleMetadata(e Entry, imageFn ImageFunction, ch chan<- IndexRecord) {
	m, err := FetchBuildpackConfig(e, imageFn)
	ch <- IndexRecord{
		metadata: m,
		entry:    e,
		err:      err,
	}
}

func FetchBuildpackConfig(e Entry, imageFn ImageFunction) (Metadata, error) {
	ref, err := name.ParseReference(e.Address)
	if err != nil {
		return Metadata{}, err
	}

	if _, ok := ref.(name.Digest); !ok {

		return Metadata{}, errors.New(fmt.Sprintf("address is not a digest: %s", e.Address))
	}

	image, err := imageFn(ref)
	if err != nil {
		return Metadata{}, err
	}

	configFile, err := image.ConfigFile()
	if err != nil {
		return Metadata{}, err
	}

	raw, ok := configFile.Config.Labels[MetadataLabel]
	if !ok {
		return Metadata{}, errors.New(fmt.Sprintf("could not find metadata label for %s", e.Address))
	}

	var m Metadata
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return Metadata{}, err
	}

	if fmt.Sprintf("%s/%s", e.Namespace, e.Name) != m.ID {
		return Metadata{}, errors.New(fmt.Sprintf("invalid ID for %s", e.Address))
	}

	if e.Version != m.Version {
		return Metadata{}, errors.New(fmt.Sprintf("invalid version for %s", e.Address))

	}

	var stacks []string
	for _, s := range m.Stacks {
		stacks = append(stacks, s.ID)
	}

	return m, nil
}

func UpdateOrInsertConfig(e Entry, m Metadata) error {

	// TODO

	//println(m.ID, m.Version)

	return nil
}
