package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/moby/term"
	"github.com/mroy31/gonetem/internal/utils"
	"github.com/sirupsen/logrus"
)

type DockerClient struct {
	cli *client.Client
}

func (c *DockerClient) Close() error {
	return c.cli.Close()
}

func (c *DockerClient) IsImagePresent(imgName string) (bool, error) {
	list, err := c.cli.ImageList(
		context.Background(),
		types.ImageListOptions{All: true})
	if err != nil {
		return false, err
	}

	for _, imgInfo := range list {
		for _, tag := range imgInfo.RepoTags {
			if tag == imgName {
				return true, nil
			}
		}
	}
	return false, nil
}

func (c *DockerClient) ImagePull(imgName string) error {
	out, err := c.cli.ImagePull(
		context.Background(),
		imgName, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer out.Close()

	io.Copy(ioutil.Discard, out)
	return nil
}

type NetemContainerList struct {
	Container types.Container
	Name      string
}

func (c *DockerClient) List(prefix string) ([]NetemContainerList, error) {
	result := make([]NetemContainerList, 0)

	list, err := c.cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return result, err
	}

	for _, container := range list {
		for _, name := range container.Names {
			if strings.HasPrefix(name, "/"+prefix) {
				result = append(result, NetemContainerList{
					Container: container,
					Name:      name[1:],
				})
				continue
			}
		}
	}

	return result, nil
}

func (c *DockerClient) Get(containerId string) (*types.Container, error) {
	list, err := c.cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}

	for _, container := range list {
		if container.ID == containerId {
			return &container, nil
		}
	}

	return nil, errors.New(fmt.Sprintf("Container with id %s does not exist", containerId))
}

func (c *DockerClient) GetState(containerId string) (string, error) {
	container, err := c.Get(containerId)
	if err != nil {
		return "", err
	}

	return container.State, nil
}

func (c *DockerClient) Create(imgName, containerName, hostName string, volumes []string, ipv6, mpls bool) (string, error) {
	hostConfig := container.HostConfig{
		NetworkMode: "none",
		Privileged:  true,
		CapAdd:      []string{"ALL"},
		Sysctls:     make(map[string]string),
		Binds:       volumes,
	}
	if ipv6 {
		hostConfig.Sysctls["net.ipv6.conf.all.disable_ipv6"] = "0"
	}
	if mpls {
		hostConfig.Sysctls["net.mpls.platform_labels"] = "100000"
		hostConfig.Sysctls["net.mpls.conf.lo.input"] = "1"
	}

	resp, err := c.cli.ContainerCreate(context.Background(), &container.Config{
		Image:    imgName,
		Hostname: hostName,
		Tty:      false,
	}, &hostConfig, nil, nil, containerName)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (c *DockerClient) Start(containerId string) error {
	return c.cli.ContainerStart(
		context.Background(), containerId, types.ContainerStartOptions{})
}

func (c *DockerClient) Stop(containerId string) error {
	state, err := c.GetState(containerId)
	if err != nil {
		return err
	}

	if state == "running" {
		timeout := time.Second * 2
		return c.cli.ContainerStop(context.Background(), containerId, &timeout)
	}

	return nil
}

func (c *DockerClient) Rm(containerId string) error {
	state, err := c.GetState(containerId)
	if err != nil {
		return err
	}

	if state == "running" {
		if err := c.Stop(containerId); err != nil {
			return err
		}
	}
	return c.cli.ContainerRemove(
		context.Background(), containerId, types.ContainerRemoveOptions{})
}

func (c *DockerClient) Pid(containerId string) (int, error) {
	containerInfo, err := c.cli.ContainerInspect(
		context.Background(), containerId)
	if err != nil {
		return -1, err
	}

	if !containerInfo.State.Running {
		return -1, errors.New(fmt.Sprintf("Container %s is not running", containerId))
	}
	return containerInfo.State.Pid, nil
}

func (c *DockerClient) IsFileExist(containerId, filepath string) bool {
	ctx := context.Background()
	sourceStat, err := c.cli.ContainerStatPath(ctx, containerId, filepath)
	if err != nil {
		return false
	}

	return sourceStat.Mode.IsRegular()
}

func (c *DockerClient) CopyFrom(containerId, source, dest string) error {
	ctx := context.Background()
	sourceStat, err := c.cli.ContainerStatPath(ctx, containerId, source)
	// we do not support the case where source is not a regular file
	if err == nil && !sourceStat.Mode.IsRegular() {
		return errors.New("DockerCopyFrom support only the copy of a regular file")
	}

	// check that Dir(dest) exists
	parentStat, err := os.Stat(path.Dir(dest))
	if err != nil || !parentStat.Mode().IsDir() {
		return errors.New(fmt.Sprintf("CopyFrom: %s is not a directory", path.Dir(dest)))
	}

	reader, _, err := c.cli.CopyFromContainer(ctx, containerId, source)
	if err != nil {
		return err
	}
	defer reader.Close()

	destPath := dest
	if stat, err := os.Stat(dest); err == nil && stat.Mode().IsDir() {
		destPath = path.Join(dest, path.Base(source))
	}
	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	srcTar := tar.NewReader(reader)
	_, err = srcTar.Next()
	if _, err = io.Copy(destFile, srcTar); err != nil {
		return err
	}

	return nil
}

func (c *DockerClient) CopyTo(containerId, source, dest string) error {
	ctx := context.Background()
	stat, err := os.Stat(source)
	if err != nil {
		return err
	} else if !stat.Mode().IsRegular() {
		return errors.New("DockerCopyTo support only the copy of a regular file")
	}

	dstStat, err := c.cli.ContainerStatPath(ctx, containerId, dest)
	// we do not support the case where dstPath is a symbolic link
	if err == nil && dstStat.Mode&os.ModeSymlink != 0 {
		return errors.New("target destination is a symlink, this case is not supported")
	}

	dstPath := path.Dir(dest)
	dstName := path.Base(dest)
	if dstStat.Mode.IsDir() {
		dstPath = dest
		dstName = path.Base(source)
	}

	// create temp tar file to use the client API
	pReader, pWriter := io.Pipe()

	go func() {
		tarWriter := tar.NewWriter(pWriter)
		if err = utils.AddFileToTar(tarWriter, source, dstName); err != nil {
			pWriter.CloseWithError(err)
			return
		}
		tarWriter.Close()
		pWriter.Close()
	}()

	return c.cli.CopyToContainer(
		context.Background(), containerId, dstPath,
		pReader, types.CopyToContainerOptions{})
}

func (c *DockerClient) Exec(containerId string, cmd []string) (string, error) {
	config := types.ExecConfig{
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          cmd,
	}

	ctx := context.Background()
	execID, err := c.cli.ContainerExecCreate(ctx, containerId, config)
	if err != nil {
		return "", err
	}

	resp, err := c.cli.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		return "", err
	}
	defer resp.Close()

	// read the output
	var outBuf, errBuf bytes.Buffer
	outputDone := make(chan error)

	go func() {
		// StdCopy demultiplexes the stream into two buffers
		_, err = stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader)
		outputDone <- err
	}()

	select {
	case err := <-outputDone:
		if err != nil {
			return "", err
		}
		break

	case <-ctx.Done():
		return "", ctx.Err()
	}

	stdout, err := ioutil.ReadAll(&outBuf)
	if err != nil {
		return "", err
	}
	stderr, err := ioutil.ReadAll(&errBuf)
	if err != nil {
		return "", err
	}

	res, err := c.cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return "", err
	}

	if res.ExitCode != 0 {
		msg := fmt.Sprintf(
			"DockerExec returns an non-zero exit code: \n\t%s", string(stderr))
		return "", errors.New(msg)
	}
	return string(stdout), nil
}

func (c *DockerClient) ExecOutStream(containerId string, cmd []string, out io.Writer) error {
	config := types.ExecConfig{
		AttachStderr: false,
		AttachStdout: true,
		Cmd:          cmd,
	}

	ctx := context.Background()
	execID, err := c.cli.ContainerExecCreate(ctx, containerId, config)
	if err != nil {
		return err
	}

	resp, err := c.cli.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		return err
	}
	defer resp.Close()

	// read the output
	var errBuf bytes.Buffer
	outputDone := make(chan error)

	go func() {
		_, err = stdcopy.StdCopy(out, &errBuf, resp.Reader)
		outputDone <- err
	}()

	select {
	case err := <-outputDone:
		if err != nil {
			return err
		}
		break

	case <-ctx.Done():
		return ctx.Err()
	}

	res, err := c.cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return err
	}

	if res.ExitCode != 0 {
		return fmt.Errorf("ExecOutStream: exit code %d", res.ExitCode)
	}
	return nil
}

func (c *DockerClient) ExecTty(containerId string, cmd []string, in io.ReadCloser, out io.Writer, resizeCh chan term.Winsize) error {
	config := types.ExecConfig{
		AttachStderr: true,
		AttachStdout: true,
		AttachStdin:  true,
		Tty:          true,
		Cmd:          cmd,
	}

	ctx := context.Background()
	execID, err := c.cli.ContainerExecCreate(ctx, containerId, config)
	if err != nil {
		return err
	}

	resp, err := c.cli.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{
		Tty: config.Tty,
	})
	if err != nil {
		return err
	}
	defer resp.Close()

	// read the output
	outputDone := make(chan error)
	go func() {
		_, err = io.Copy(out, resp.Reader)
		outputDone <- err
	}()

	// write the input
	inputDone := make(chan error)
	go func() {
		_, err := io.Copy(resp.Conn, in)
		inputDone <- err
	}()

	// resize TTY goroutine
	go func() {
		for ws := range resizeCh {
			if err := c.cli.ContainerExecResize(ctx, execID.ID, types.ResizeOptions{
				Height: uint(ws.Height),
				Width:  uint(ws.Width),
			}); err != nil {
				logrus.WithField(
					"container",
					containerId,
				).Errorf("unable to resize TTY: %s", err)
			}
		}
	}()

	select {
	case err := <-outputDone:
		if err != nil {
			return err
		}
		break

	case <-inputDone:
		// Input stream has closed.
		// Wait for output to complete streaming.
		select {
		case err := <-outputDone:
			if err != nil {
				return err
			}
			break
		case <-ctx.Done():
			return ctx.Err()
		}

	case <-ctx.Done():
		return ctx.Err()
	}

	res, err := c.cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return err
	}

	if res.ExitCode != 0 {
		return fmt.Errorf("ExecTty: exit code %d", res.ExitCode)
	}
	return nil
}

func NewDockerClient() (*DockerClient, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	return &DockerClient{
		cli: cli,
	}, nil
}
