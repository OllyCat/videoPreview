package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/schollz/progressbar"

	"gocv.io/x/gocv"
)

func main() {
	// подсказка по использованию
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Using:\n\n%s <file name> [ <file name> ... ]\n", os.Args[0])
	}

	// парсинг флагов
	flag.Parse()
	if len(flag.Args()) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	// пробегаем в цикле по всем входным файлам
	for _, fName := range flag.Args() {
		ext := path.Ext(fName)
		pName := strings.TrimSuffix(fName, ext) + ".preview.jpg"
		// открываем файл
		f, err := gocv.OpenVideoCapture(fName)
		if err != nil {
			log.Fatal(err)
		}

		// количество кадров в превьюшке
		sQuant := 25
		// массив кадров
		imgs := make([]image.Image, 0, sQuant)
		// стартовый кадр: 0
		var vPos int64
		// получаем количество кадров
		quant := int64(f.Get(gocv.VideoCaptureFrameCount))
		// поучаем ширину кадра
		w := f.Get(gocv.VideoCaptureFrameWidth)
		// получаем высоту кадра
		h := f.Get(gocv.VideoCaptureFrameHeight)
		// получаем множитель для уменьшения кадра до 320
		denom := w / 320
		// получаем прирост кадров, что бы получить с любого видео нужное количество кадров
		vDelta := quant / int64(sQuant)

		// печатаем информацию
		fmt.Printf("Total number of frames: %v\nNumber of screenshots: %v\n\n", quant, sQuant)

		// создаём прогресс бар
		bar := progressbar.New(sQuant)

		for i := 0; i < sQuant; i++ {
			// встаём на очередную позицию в видеопотоке
			f.Set(gocv.VideoCapturePosFrames, float64(vPos))
			// смещаемся на дельту
			vPos += vDelta
			// берём новый mat
			mat := gocv.NewMat()

			// читаем кадр в mat
			ok := f.Read(&mat)
			// если не прочитался - выходим
			if !ok {
				bar.Finish()
				log.Println("WARN: Битый видео поток.")
				break
			}

			// изменяем размер мата
			gocv.Resize(mat, &mat, image.Point{int(w / denom), int(h / denom)}, 0, 0, gocv.InterpolationLinear)

			img, err := mat.ToImage()
			if err != nil {
				log.Printf("Frame %d has error: %v\n", i, err)
				continue
			}

			// собираем полученные картинки в слайс
			imgs = append(imgs, img)
			// крутим бар
			bar.Add(1)
		}

		// монтируем картинку
		//err = montageShell(imgs, fName, pName)
		err = montageNative(imgs, fName, pName)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\nDone: %s\n\n", pName)
	}
}

// сбор картинок в одну с помощью внешней комманды imagemagick
func montageShell(imgs []image.Image, fName, pName string) (errRet error) {
	// для этого приходится сохранить все картинки в файлы
	for i, img := range imgs {
		tmpFName := fmt.Sprintf("out%02d.jpg", i)
		f, err := os.Create(tmpFName)
		if err != nil {
			log.Fatalf("Could not create temp file: %v\n", err)
		}
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
		f.Close()
		if err != nil {
			log.Fatalf("Could not encode to jpeg: %v\n", err)
		}
	}

	cmd := exec.Command("montage", "-shadow", "-frame", "5", "-geometry", "+10+10", "out??.jpg", pName)
	errRet = cmd.Run()
	for i, _ := range imgs {
		tmpFName := fmt.Sprintf("out%02d.jpg", i)
		os.Remove(tmpFName)
	}
	return
}

// сбор картинок в одну с помощью библиотеки image
func montageNative(imgs []image.Image, fName, pName string) (err error) {

	// размер превьюшки
	b := imgs[0].Bounds()

	// размер общей превьюшки
	l := len(imgs)
	rx := 5
	if l < 5 {
		rx = l
	}
	ry := l / 5
	if l%5 > 0 {
		ry++
	}
	r := image.Rect(0, 0, b.Dx()*rx+(rx+1)*5, b.Dy()*ry+(ry+1)*5)

	// создаём картинку
	prw := image.NewRGBA(r)
	// фон
	c := color.RGBA{255, 255, 255, 255}

	// размер превью
	bo := prw.Bounds()

	// красим в цвет фона
	draw.Draw(prw, bo, &image.Uniform{c}, image.ZP, draw.Src)

	// дельты для каждого кадра
	xd := 0
	yd := 0

	for i, img := range imgs {
		// смещение для каждого кадра относительное
		x := i % 5
		y := i / 5
		// смещение для каждого кадра абсолютное
		xd = x*(b.Dx()+5) + 5
		yd = y*(b.Dy()+5) + 5

		// крисуем очередной кадр
		draw.Draw(prw, b.Add(image.Pt(xd, yd)), img, image.ZP, draw.Over)
	}

	// открываем файл
	f, err := os.Create(pName)
	if err != nil {
		return
	}

	// кодируем в jpg
	err = jpeg.Encode(f, prw, nil)
	if err != nil {
		return
	}
	return
}
