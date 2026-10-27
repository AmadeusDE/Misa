package vm

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"

	"github.com/AmadeusDE/Misa/internal/ughbot/config"
)

type boot struct {
	disk    string
	serial  string
	sshPort int // forwarded from 127.0.0.1 to the guest's port 22; 0 forwards nothing
	shares  []string
	smbios  string // a NoCloud seed for cloud-init
}

func kvm() bool {
	f, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func qemuArgs(v config.VM, b boot) []string {
	args := []string{"-nodefaults", "-no-user-config", "-no-reboot", "-display", "none", "-monitor", "none",
		"-machine", "q35", "-m", strconv.Itoa(v.MemoryMB), "-smp", strconv.Itoa(v.CPUs),
		"-serial", "file:" + b.serial,
		"-drive", "file=" + b.disk + ",if=virtio,format=qcow2,discard=unmap",
		"-device", "virtio-rng-pci"}
	if kvm() {
		args = append(args, "-accel", "kvm", "-cpu", "host")
	} else {
		args = append(args, "-accel", "tcg", "-cpu", "max")
	}
	netdev := "user,id=n0"
	if b.sshPort != 0 {
		netdev += fmt.Sprintf(",hostfwd=tcp:127.0.0.1:%d-:22", b.sshPort)
	}
	if !*v.Network {
		netdev += ",restrict=on"
	}
	args = append(args, "-netdev", netdev, "-device", "virtio-net-pci,netdev=n0")
	for i, dir := range b.shares {
		args = append(args, "-virtfs", fmt.Sprintf("local,path=%s,mount_tag=%s,security_model=none,multidevs=remap", dir, shareTag(i)))
	}
	if b.smbios != "" {
		args = append(args, "-smbios", "type=1,serial=ds=nocloud;s="+b.smbios+"/")
	}
	return args
}

func shareTag(i int) string { return "share" + strconv.Itoa(i) }

type qemuProc struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
}

func startQEMU(args []string, logPath string) (*qemuProc, error) {
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("qemu-system-x86_64", args...)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = qemuProcAttr()
	if err := cmd.Start(); err != nil {
		log.Close()
		return nil, fmt.Errorf("qemu: %w", err)
	}
	p := &qemuProc{cmd: cmd, done: make(chan struct{})}
	go func() {
		p.err = cmd.Wait()
		log.Close()
		close(p.done)
	}()
	return p, nil
}

func (p *qemuProc) kill() {
	_ = p.cmd.Process.Kill()
	<-p.done
}

func qemuImg(args ...string) error {
	out, err := exec.Command("qemu-img", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("qemu-img %s: %v: %s", args[0], err, out)
	}
	return nil
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}
