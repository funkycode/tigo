package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/xinerama"
	"github.com/BurntSushi/xgbutil/xrect"
	"github.com/BurntSushi/xgbutil/xwindow"

	"github.com/spf13/cobra"
)

func getHeads() (heads xinerama.Heads, err error) {
	X, err := xgbutil.NewConn()
	if err != nil {
		log.Fatal(err)
	}

	// Wrap the root window in a nice Window type.
	root := xwindow.New(X, X.RootWin())

	// Get the geometry of the root window.
	rgeom, err := root.Geometry()
	if err != nil {
		log.Fatal(err)
	}

	// Get the rectangles for each of the active physical heads.
	// These are returned sorted in order from left to right and then top
	// to bottom.
	// But first check if Xinerama is enabled. If not, use root geometry.
	//var heads xinerama.Heads
	if X.ExtInitialized("XINERAMA") {
		heads, err = xinerama.PhysicalHeads(X)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		heads = xinerama.Heads{rgeom}
	}

	// Fetch the list of top-level client window ids currently managed
	// by the running window manager.
	clients, err := ewmh.ClientListGet(X)
	if err != nil {
		log.Fatal(err)
	}
	// For each client, check to see if it has struts, and if so, apply
	// them to our list of head rectangles.
	for _, clientid := range clients {
		strut, err := ewmh.WmStrutPartialGet(X, clientid)
		if err != nil { // no struts for this client
			continue
		}

		// Apply the struts to our heads.
		// This modifies 'heads' in place.
		xrect.ApplyStrut(heads, uint(rgeom.Width()), uint(rgeom.Height()),
			strut.Left, strut.Right, strut.Top, strut.Bottom,
			strut.LeftStartY, strut.LeftEndY,
			strut.RightStartY, strut.RightEndY,
			strut.TopStartX, strut.TopEndX,
			strut.BottomStartX, strut.BottomEndX)
	}
	return
}

func getCurrentInfo() (currentWindow *xwindow.Window, currentHead xrect.Rect, err error) {
	heads, err := getHeads()
	if err != nil {
		return
	}
	X, err := xgbutil.NewConn()
	if err != nil {
		return
	}
	currentWindowID, err := ewmh.ActiveWindowGet(X)
	currentWindow = xwindow.New(X, currentWindowID)
	parent, err := currentWindow.Parent()
	if err != nil {
		return
	}
	parentGeometry, err := parent.Geometry()
	if err != nil {
		return
	}
	fmt.Printf("Geometry: %#v\n", parentGeometry)
	var currentIntersection int
	for _, head := range heads {
		intersection := xrect.IntersectArea(head, parentGeometry)
		if intersection == 0 {
			continue
		}
		if intersection > currentIntersection {
			currentHead = head
			currentIntersection = intersection
		}
	}
	return
}

func fillInLayout(xDiv uint, yDiv uint, xSize uint, ySize uint, xIndx uint, yIndx uint) (err error) {
	if yIndx > yDiv {
		err = fmt.Errorf("Can't place out of Y")
		return
	}
	currentWindow, currentHead, err := getCurrentInfo()
	if err != nil {
		return
	}
	if currentHead == nil {
		err = fmt.Errorf("Can't find suitable screen")
		return
	}
	geom, err := currentWindow.Geometry()
	if err != nil {
		return
	}
	var x, y, w, h int
	w = currentHead.Width() / int(xDiv) * int(xSize)
	h = currentHead.Height() / int(yDiv) * int(ySize)
	x = currentHead.X() + int(xIndx-1)*currentHead.Width()/int(xDiv)
	y = currentHead.Y() + int(yIndx-1)*(currentHead.Height()/int(yDiv))
	if y+h+geom.Y() > currentHead.Y()+currentHead.Height() {
		y = currentHead.Y() + currentHead.Height() - h - geom.Y()
	}
	if x+w+geom.X() > currentHead.X()+currentHead.Width() {
		x = currentHead.X() + currentHead.Width() - w - geom.X()
	}
	currentHead.Pieces()
	err = currentWindow.WMMoveResize(x, y, w, h)
	return
}
func main() {

	cmd := &cobra.Command{
		Use:   "tigo",
		Short: "Tile window by grid",
		Args:  cobra.ExactArgs(6),
		Run: func(cmd *cobra.Command, args []string) {
			var xDiv, yDiv, xSize, ySize, xIndx, yIndx uint
			argsParsed := []*uint{&xDiv, &yDiv, &xSize, &ySize, &xIndx, &yIndx}

			for i, arg := range args {
				intArg, err := strconv.ParseUint(arg, 10, 32)
				if err != nil {
					fmt.Println("Invalid arg:", arg, ". Error:", err.Error())
					return
				}
				argUint := uint(intArg)
				*argsParsed[i] = argUint
			}
			err := fillInLayout(xDiv, yDiv, xSize, ySize, xIndx, yIndx)
			if err != nil {
				fmt.Println("Error:", err)
			}

		},
	}
	if err := cmd.Execute(); err != nil {
		fmt.Println("Error:", err.Error())
	}

}
