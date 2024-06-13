package main

import (
	"bufio"
	"crypto/tls"
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/i582/cfmt/cmd/cfmt"
	"github.com/nathan-fiscaletti/consolesize-go"
)

// общая константа размера буффера сообщений
const bufferMessSize byte = 64

// зарезервированное спец.имя клиента
const clientName string = "client"

// зарезервированное спец.имя сервера
const serverName string = "server"

// структура сообщения
type Message struct {
	Sender   string //отправитель
	Receiver string //получатель
	Text     string //текст
}

// название
const AsciiTitle string = `
		Yet Another Messanger

	Y88b   d88P      d8888   888b     d888
	 Y88b d88P      d88888   8888b   d8888
	  Y88o88P      d88P888   88888b.d88888
	   Y888P      d88P 888   888Y88888P888 
	    888      d88P  888   888 Y888P 888
	    888     d888888888   888  Y8P  888
	    888    d88P    888   888       888
	    888    d888    888   888       888`

// автор
const Copyright string = "Ilya \"Update56\" Cherdakov  AtlSTU 2024"

func main() {
	//загружаем пару сертификат/ключ
	cer, err := tls.LoadX509KeyPair("cert/client.crt", "cert/client.key")
	if err != nil {
		fmt.Println(err)
		return
	}
	//создаём конфигурацию
	config := &tls.Config{Certificates: []tls.Certificate{cer}, InsecureSkipVerify: true}
	//вывод названия
	titlePrint()
	//ввод никнейма
	fmt.Println("Введите никнейм")
	var nickname string
	//проверка на имя пользователя, чтобы оно не совпадало с зарезервированными именами
	for {
		fmt.Scan(&nickname)
		if strings.ToLower(nickname) != clientName &&
			strings.ToLower(nickname) != serverName {
			break
		}
		fmt.Println("Это зарезервированное имя. Введите другое")
	}

	//ввод IP-адреса:порт сервера
	fmt.Println("Введите IP:port сервера")
	var ipAddress string
	fmt.Scan(&ipAddress)

	//установка соединение с сервером по введённому <IP:port> с tls
	conn, err := tls.Dial("tcp", ipAddress, config)
	//проверка на ошибку при установке
	if err != nil {
		fmt.Println(err)
		return
	}
	//закрытие в конце выполнения программы
	defer conn.Close()
	//нулевое сообщение
	sendMessage(conn, nickname, serverName, "")

	//создание хэш-таблицы ключ - никнейм отправителя, значение - канал
	var usersMap = map[string]chan string{
		serverName: make(chan string, bufferMessSize), //нулевая запись со спец.именем сервера
	}
	//пустой список пользователей в сети
	var usersOnline []string
	//запуск функции обработки служебных сообщений
	go procSpecialMessages(usersMap[serverName], &usersOnline)

	//запуск получения сообщений
	go receivingMessage(conn, usersMap)

	//ожидаем пока получим список ввода
	for {
		fmt.Println("В онлайне никого нет...")
		time.Sleep(time.Second)
		clearCons()
		//пока список пуст, пропускаем цикл
		if len(usersOnline) == 0 {
			continue
		}
		break
	}

	for {
		clearCons()
		//вывод списка пользователей онлайн
		printList(usersOnline, nickname)
		//установка курсора в самую нижнию строку (строка ввода)
		_, y := consolesize.GetConsoleSize()
		setCursorPosition(0, y-1)
		//считываем имя пользователя для общения
		var user string
		fmt.Scan(&user)
		//проверка на выход
		if len(user) >= 3 {
			if user[:3] == "###" {
				procExit(conn, nickname)
			}
		}
		//проверка
		if idx := slices.Index(usersOnline, user); idx == -1 || user == nickname {
			setCursorPosition(0, len(usersOnline)+1)
			fmt.Print("\tТакого пользователя нет!")
			time.Sleep(time.Second)
			setCursorPosition(consolesize.GetConsoleSize())
			continue
		}
		//проверка на наличие в хэш-таблице
		if _, available := usersMap[user]; !available {
			//если записи нет, создать
			usersMap[user] = make(chan string, bufferMessSize)
		}
		//диалог
		procDiaolog(nickname, user, usersMap[user], conn)
	}
}

// функция диалога с пользователем
func procDiaolog(myName string, recName string, buffChan chan string, conn net.Conn) {
	//очистка консоли
	clearCons()
	//очищаем стандартный поток
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	//счётчик строк для правильного отображения сообщений
	strCount := 1
	//переменная для ввода строки
	var input string
	//анонимная функция для вывода сообщения из канала

	for len(buffChan) != 0 {
		printMessage(recName, &strCount, <-buffChan)
	}

	go func() {
		//проверка на символы выхода из диалога
		for input != "###" {
			//проверка на пустоту канала, чтобы поток не блокировался
			if len(buffChan) == 0 {
				continue
			}
			//вывод сообщения
			printMessage(recName, &strCount, <-buffChan)
		}
	}()

	for {
		_, y := consolesize.GetConsoleSize()
		setCursorPosition(0, y)
		cfmt.Print("{{\"###\" - выход из дилога  \"Enter - ввод\"}}::bgWhite|#000000")
		//установка курсора в нижнию строку (строка ввода)
		setCursorPosition(0, y-1)
		//чтение ввода со стандартного ввода(консоли)
		input, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		//последовательность для выхода
		if len(input) >= 3 {
			if input[:3] == "###" {
				time.Sleep(time.Second)
				return
			}
		}

		//выводим написанное сообщение
		printMessage(myName, &strCount, input)
		//очищаем поле ввода
		fmt.Printf("\033[A\033[1G\033[2K")
		//обрезаем символ перевода строки
		input = input[:len(input)-1]
		//отправляем сообщение на сервер
		sendMessage(conn, myName, recName, input)
	}
}

func printMessage(name string, strCount *int, text string) {
	//сохраняем текущее положение курсора в строке ввода
	fmt.Printf("\033[s")
	//устанавливаем курсор на свободную строку
	setCursorPosition(0, *strCount)
	//выводим сообщение
	fmt.Print(name, ": ", text)
	//увеличиваем кол-во строк
	*strCount++
	//если строки займут весь экран, очистить терминал, кроме строки ввода
	if _, y := consolesize.GetConsoleSize(); *strCount >= y-2 {
		clearCons()
	}
	//востанавливаем курсор в строке ввода
	fmt.Printf("\033[u")
}

// функция отправки сообщений
func sendMessage(conn net.Conn, myName string, recName string, text string) {
	//объявляем выходной енкодер с потоком в виде подключения
	enc := gob.NewEncoder(conn)
	//передача сообщения о получении списка пользователей (сообщение сделать константой)
	enc.Encode(Message{myName, recName, text})
}

// функция получения сообщений и распределения
func receivingMessage(conn net.Conn, usersMap map[string]chan string) {
	//цикл получения и распределения сообщений
	for {
		//объявляем входной декодер с потоком в виде подключения
		dec := gob.NewDecoder(conn)

		//пустая структура для сообщения
		recMess := Message{}

		//десериализация полученного сообщения
		dec.Decode(&recMess)

		//проверка на наличие пользователя в хэш-таблице(в отдельную функцию)
		if _, available := usersMap[recMess.Sender]; !available {
			//если записи нет, создать
			usersMap[recMess.Sender] = make(chan string, bufferMessSize)
		}
		//отправляем сообщение в канал, соотвествующий никнейму отправителя
		usersMap[recMess.Sender] <- recMess.Text
	}
}

// функция обработки служебных сообщений
func procSpecialMessages(buffChan chan string, usersOnline *[]string) {
	for {
		//получение текста из канала
		text := <-buffChan
		switch text[:3] {
		//001 - получение списка пользователей
		case "001":
			//Отсекаем код операции и \n
			text = text[4:]
			//делим строку на слайс с никами юзеров в онлайне
			*usersOnline = strings.Split(text, "\n")
		}
	}
}

// функция получения списка пользователей в сети
func getOnlineUsers(conn net.Conn) {
	//вызов отправки сообщения со спец.командой
	sendMessage(conn, clientName, serverName, "GetOnlineList")
}

// функция выхода
func procExit(conn net.Conn, nickname string) {
	sendMessage(conn, nickname, serverName, "Disconnect")
	os.Exit(1)
}

// функция вывода списка онлайна
func printList(usersOnline []string, nickname string) {
	setCursorPosition(0, 0)
	fmt.Println("Пользователи онлайн")
	for _, user := range usersOnline {
		if user == nickname {
			fmt.Print("(Вы) ")
		}
		fmt.Println(user)

	}
	_, y := consolesize.GetConsoleSize()
	setCursorPosition(0, y)
	cfmt.Printf("{{Введите никнейм чтобы начать диалог  \"###\" - выход}}::bgWhite|#000000")
}

// функция установки курсора на позицию x,y
func setCursorPosition(x, y int) {
	fmt.Printf("\033[%d;%dH", y, x)
}

// вывод названия
func titlePrint() {
	_, yT := consolesize.GetConsoleSize()
	fmt.Println(AsciiTitle)
	setCursorPosition(0, yT-1)
	fmt.Print(Copyright)
	time.Sleep(time.Second * 3)
	clearCons()
}

// очистка консоли
func clearCons() {
	setCursorPosition(0, 0)
	fmt.Printf("\033[0J")
}
