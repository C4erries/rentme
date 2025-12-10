import csv
import numpy as np
import random


def square_trick(weights, bias, data, label, n):
    if len(data) != len(weights):
        print('Data and weight dimensions do not match')
        exit()
    weights = np.array(weights)
    data = np.array(data)
    predict = np.sum(weights * data) + bias
    weights += n * data * (label - predict)
    bias += n * (label - predict)
    return weights, bias


def liner_regression(data, labels, n=0.00001, epochs=100000):
    bias = 0
    weights = np.array([1.0 for _ in range(len(data[0]))])
    for _ in range(epochs):
        i = random.randint(0, len(data) - 1)
        point = data[i]
        label = labels[i]
        weights, bias = square_trick(weights, bias, point, label, n)
        weights[weights < 0] += 0.1
        weights[weights > 0] -= 0.1
        if bias < 0:
            bias += 0.1
        elif bias > 0:
            bias -= 0.1
    return weights, bias


def parser(file):
    rows = []
    with open(file, 'r', encoding='utf-8', newline='') as f:
        reader = csv.DictReader(f)
        for row in reader:
            rows.append(row)

    way_map = {'walk': 0, 'car': 1}
    data = []
    labels = []
    for row in rows:
        label = float(row['price']) / 1000  # predict price in thousands
        way = way_map.get(row['way'].strip().lower(), 0)
        features = [
            float(row['minutes']),
            float(way),
            float(row['rooms']),
            float(row['total_area']),
            float(row['storey']),
            float(row['storeys']),
            float(row['renovation']),
            float(row['building_age_years']),
        ]
        data.append(features)
        labels.append(label)

    data = np.array(data, dtype=float)
    labels = np.array(labels, dtype=float)

    indices = np.arange(len(data))
    np.random.shuffle(indices)
    data = data[indices]
    labels = labels[indices]

    split_index = int(len(data) * 0.1)
    test_data = data[:split_index]
    test_labels = labels[:split_index]
    data = data[split_index:]
    labels = labels[split_index:]
    return data, labels, test_data, test_labels


def test(weights, bias, test_l, test_d):
    weights = np.array(weights)
    test_d = np.array(test_d)
    p = 0
    print('Predicted vs actual (price in thousands)')
    for i in range(len(test_d)):
        predict = np.sum(weights * test_d[i]) + bias
        print(predict, test_l[i])
        p += abs(predict - test_l[i])
    p /= len(test_d)
    print(p)


data, labels, test_data, test_labels = parser('mlrent/clean_train.csv')

weights, bias = liner_regression(data, labels)

test(weights, bias, test_labels, test_data)

print(weights, bias)
