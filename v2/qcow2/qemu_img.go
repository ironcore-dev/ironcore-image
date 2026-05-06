// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package qcow2

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type QemuImg struct {
	name string
}

func NewQemuImg(name string) *QemuImg {
	return &QemuImg{name}
}

func DefaultQemuImg() *QemuImg {
	return NewQemuImg("qemu-img")
}

func (q *QemuImg) UnsafeRemoveBacking(filename string) error {
	args := []string{"rebase", "--backing", "", "--backing-unsafe", filename}
	res, err := exec.Command(q.name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("rebasing image: %w (%s)", err, string(res))
	}
	return nil
}

type blockdevOptions struct {
	Driver                string           `json:"driver"`
	File                  *blockdevOptions `json:"file,omitempty"`
	*blockdevOptionsFile  `json:",inline,omitempty"`
	*blockdevOptionsQcow2 `json:",inline,omitempty"`
}

type blockdevOptionsFile struct {
	Filename string `json:"filename"`
}

type blockdevOptionsQcow2 struct {
	Backing *blockdevOptions `json:"backing"`
}

func (q *QemuImg) chainImageOpts(chain []string) (string, error) {
	var res *blockdevOptions
	cur := &res
	for i := len(chain) - 1; i >= 0; i-- {
		c := chain[i]
		*cur = &blockdevOptions{
			Driver: "qcow2",
			File: &blockdevOptions{
				Driver: "file",
				blockdevOptionsFile: &blockdevOptionsFile{
					Filename: c,
				},
			},
			blockdevOptionsQcow2: &blockdevOptionsQcow2{},
		}

		cur = &(*cur).Backing
	}

	imageOptsJSONData, err := json.Marshal(*res)
	if err != nil {
		return "", fmt.Errorf("marshalling image opts: %w", err)
	}

	return fmt.Sprintf("json:%s", string(imageOptsJSONData)), nil
}

func (q *QemuImg) WriteChain(chain []string, filename string) error {
	imageOpts, err := q.chainImageOpts(chain)
	if err != nil {
		return fmt.Errorf("getting image opts: %w", err)
	}

	args := []string{"convert", "--image-opts", "--no-create", "-t", "none", "-T", "none", imageOpts, filename}
	res, err := exec.Command(q.name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("writing chain: %w (%s)", err, string(res))
	}
	return nil
}
