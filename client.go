package main

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"net"
	"os"
	"slices"
	"strings"
	"time"

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
const AsciiTitle string = "\n\t\tYet Another Messanger\n\n" +
	"\tY88b   d88P      d8888   888b     d888\n" +
	"\t Y88b d88P      d88888   8888b   d8888\n" +
	"\t  Y88o88P      d88P888   88888b.d88888\n" +
	"\t   Y888P      d88P 888   888Y88888P888\n" +
	"\t    888      d88P  888   888 Y888P 888\n" +
	"\t    888     d88P   888   888  Y8P  888\n" +
	"\t    888    d8888888888   888       888\n" +
	"\t    888   d88P     888   888       888\n"

const Copyright string = "Ilya \"Update56\" Cherdakov  AtlSTU 2024"

func main() {
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

	//установка соединение с сервером по введённому <IP:port>
	conn, err := net.Dial("tcp", ipAddress)
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
		//пока список пуст, пропускаем цикл
		if len(usersOnline) == 0 {
			continue
		}
		break
	}

	for {
		//очищаем териминал
		fmt.Printf("\033[1J")
		//вывод списка пользователей онлайн
		printList(usersOnline)
		//установка курсора в самую нижнию строку (строка ввода)
		_, y := consolesize.GetConsoleSize()
		setCursorPosition(0, y-1)
		//считываем имя пользователя для общения
		var user string
		fmt.Scan(&user)

		//проверка (в отдельную функцию)
		if idx := slices.Index(usersOnline, user); idx == -1 || user == nickname {
			setCursorPosition(0, len(usersOnline)+1)
			fmt.Printf("Такого пользователя нет!")
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
	//счётчик строк для правильного отображения сообщений
	strCount := 1
	//переменная для ввода строки
	var input string

	//анонимная функция для вывода сообщения из канала
	go func() {
		//проверка на символы выхода из диалога
		for input == "###" {
			//проверка на пустоту канала, чтобы поток не блокировался
			if len(buffChan) == 0 {
				continue
			}
			printMessage(recName, &strCount, <-buffChan)
		}
	}()

	for {
		//чтение ввода со стандартного ввода(консолм)
		input, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		//последовательность для выхода
		if input[:3] == "###" {
			time.Sleep(time.Second)
			return
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
		setCursorPosition(0, y-1)
		fmt.Printf("\033[1J")
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

	//объявляем входной декодер с потоком в виде подключения
	dec := gob.NewDecoder(conn)

	//цикл получения и распределения сообщений
	for {
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

// функция вывода списка онлайна
func printList(usersOnline []string) {
	setCursorPosition(0, 0)
	fmt.Println("Пользователи онлайн")
	for _, user := range usersOnline {
		fmt.Println(user)
	}
}

// функция установки курсора на позицию x,y
func setCursorPosition(x, y int) {
	fmt.Printf("\033[%d;%dH", y, x)
}

// выводназвания
func titlePrint() {
	xT, yT := consolesize.GetConsoleSize()
	fmt.Print(AsciiTitle)
	setCursorPosition(xT-len(Copyright), yT)
	fmt.Print(Copyright)
	time.Sleep(time.Second * 3)
	fmt.Printf("\033[1J")
	setCursorPosition(0, 0)
}

//2 решения: 1) топорное: ввод определённого набора символов (например: "###")
//2) через UNIX-сигналы: при нажатии сочетания клавиш ctrl+c (^C) будет произведён выход из диалога в главное меню
