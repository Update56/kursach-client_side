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

func main() {

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

	//создание хэш-таблицы ключ - никнейм, значение - канал
	var usersMap = map[string]chan string{
		clientName: make(chan string, bufferMessSize), //нулевая запись со спец.именем
	}
	//пустой список пользователей в сети
	var usersOnline []string
	//запуск функции обработки служебных сообщений
	go procSpecialMessages(usersMap[clientName], &usersOnline)

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
		fmt.Printf("\033[2J")
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
		sendingMessage(conn, myName, recName, input)
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
func sendingMessage(conn net.Conn, myName string, recName string, text string) {
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
			//Отсекаем код операции
			text = text[4:]
			//делим одну строку на слайс
			*usersOnline = strings.Split(text, "\n")
		}
	}
}

// функция получения списка пользователей в сети
func getOnlineUsers(conn net.Conn) {
	//вызов отправки сообщения со спец.командой
	sendingMessage(conn, clientName, serverName, "GetOnlineList")
}

// функция вывода списка онлайна
func printList(usersOnline []string) {
	for _, user := range usersOnline {
		fmt.Println(user)
	}
}

// функция установки курсора на позицию x,y
func setCursorPosition(x, y int) {
	fmt.Printf("\033[%d;%dH", y, x)
}

//2 решения: 1) топорное: ввод определённого набора символов (например: "###")
//2) через UNIX-сигналы: при нажатии сочетания клавиш ctrl+c (^C) будет произведён выход из диалога в главное меню
