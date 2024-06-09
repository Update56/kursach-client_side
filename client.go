package main

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"net"
	"strings"
	"sync"
)

// общая константа размера буффера сообщений
const bufferMessSize byte = 64

// зарезервированное имя клиента
const clientName string = "client"

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
	//проверка на имя пользователя, чтобы оно не совпадало с зарезервированным именем
	for {
		fmt.Scan(&nickname)
		if strings.ToLower(nickname) != clientName {
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

	//создание хэш-таблицы ключ - никнейм, значение - канал c единственной
	var usersList = map[string]chan string{
		clientName: make(chan string, bufferMessSize), //нулевая запись со спец.именем
	}

	go receivingMessage(conn, usersList)
	//conn.Write([]byte(nickname))

	//объявляем поток вывода с источником в виде подключения
	buf := bufio.NewReader(conn)

	line := make([]byte, 1024)
	//считываем поток байтов
	n, err := buf.Read(line)
	//проверка на ошибку
	if n == 0 || err != nil {
		fmt.Println("Read error:", err)
		return
	}
	fmt.Println(string(line[:n]))
	//waitGroup для синхронизации горутин
	var wg sync.WaitGroup
	wg.Add(2)
	//отправка сообщений
	go sendingMessage(conn, &wg)
	//получение сообщений
	wg.Wait()
}

// функция отправки сообщений
func sendingMessage(conn net.Conn, wg *sync.WaitGroup) {
	for {
		var text string
		fmt.Scan(&text)
		if text == "###" {
			wg.Done()
			break
		}
		conn.Write([]byte(text))
	}
}

// функция получения сообщений и распределения
func receivingMessage(conn net.Conn, usersList map[string]chan string) {

	//объявляем входной декодер с источником в виде подключения
	dec := gob.NewDecoder(conn)

	//цикл получения и распределения сообщений
	for {
		//пустая структура для сообщения
		recMess := Message{}

		//десериализация полученного сообщения
		dec.Decode(&recMess)

		//проверка на наличие пользователя в хэш-таблице
		if _, available := usersList[recMess.Sender]; !available {
			//если записи нет, создать
			usersList[recMess.Sender] = make(chan string, bufferMessSize)
		}
		//отправляем сообщение в канал, соотвествующий никнейму отправителя
		usersList[recMess.Sender] <- recMess.Text
	}
}

//функции "sendingMessage" и "receivingMessage" будут выполняться
//в отдельных горутинах чтобы не блокировать ввод и вывод

//проблема:нужно как-то выходить из диалога
//2 решения: 1) топорное: ввод определённого набора символов (например: "###")
//2) через UNIX-сигналы: при нажатии сочетания клавиш ctrl+c (^C) будет произведён выход из диалога в главное меню
